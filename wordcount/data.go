package main

import (
	"encoding/json"
	"fmt"
	"github.com/pauloaguiar/ces27-lab1/mapreduce"
	"io/ioutil"
	"log"
	"os"
    "io"
	"path/filepath"
    "unicode"
    "unicode/utf8"
)

const (
	MAP_PATH           = "map/"
	RESULT_PATH        = "result/"
	MAP_BUFFER_SIZE    = 10
	REDUCE_BUFFER_SIZE = 10
)

// fanInData will run a goroutine that reads files crated by splitData and share them with
// the mapreduce framework through the one-way channel. It'll buffer data up to
// MAP_BUFFER_SIZE (files smaller than chunkSize) and resume loading them
// after they are read on the other side of the channle (in the mapreduce package)
func fanInData(numFiles int) chan []byte {
	var (
		err    error
		input  chan []byte
		buffer []byte
	)

	input = make(chan []byte, MAP_BUFFER_SIZE)

	go func() {
		for i := 0; i < numFiles; i++ {
			if buffer, err = ioutil.ReadFile(mapFileName(i)); err != nil {
				close(input)
				log.Fatal(err)
			}

			log.Println("Fanning in file", mapFileName(i))
			input <- buffer
		}
		close(input)
	}()
	return input
}

// fanOutData will run a goroutine that receive data on the one-way channel and will
// proceed to store it in their final destination. The data will come out after the
// reduce phase of the mapreduce model.
func fanOutData() (output chan []mapreduce.KeyValue, done chan bool) {
	var (
		err           error
		file          *os.File
		fileEncoder   *json.Encoder
		reduceCounter int
	)

	output = make(chan []mapreduce.KeyValue, REDUCE_BUFFER_SIZE)
	done = make(chan bool)

	go func() {
		for v := range output {
			log.Println("Fanning out file", resultFileName(reduceCounter))
			if file, err = os.Create(resultFileName(reduceCounter)); err != nil {
				log.Fatal(err)
			}

			fileEncoder = json.NewEncoder(file)

			for _, value := range v {
				fileEncoder.Encode(value)
			}

			file.Close()
			reduceCounter++
		}

		done <- true
	}()

	return output, done
}

// Reads input file and split it into files smaller than chunkSize.
// CUTCUTCUTCUTCUT!
func splitData(fileName string, chunkMaxSize int) (numMapFiles int, err error) {
	// 	When you are reading a file and the end-of-file is found, an error is returned.
	// 	To check for it use the following code:
	// 		if bytesRead, err = file.Read(buffer); err != nil {
	// 			if err == io.EOF {
	// 				// EOF error
	// 			} else {
	//				return 0, err
	//			}
	// 		}
	//
	// 	Use the mapFileName function to generate the name of the files!
	//
	//	Go strings are encoded using UTF-8.
	// 	This means that a character can't be handled as a byte. There's no char type.
	// 	The type that hold's a character is called a 'rune' and it can have 1-4 bytes (UTF-8).
	//	Because of that, it's not possible to index access characters in a string the way it's done in C.
	//		str[3] will return the 3rd byte, not the 3rd rune in str, and this byte may not even be a valid
	//		rune by itself.
	//		Rune handling should be done using the package 'strings' (https://golang.org/pkg/strings/)
	//
	//  For more information visit: https://blog.golang.org/strings
	//
	//	It's also important to notice that errors can be handled here or they can be passed down
	// 	to be handled by the caller as the second parameter of the return.

	/////////////////////////
	// YOUR CODE GOES HERE //
	/////////////////////////
    
    chunkSize := int64(chunkMaxSize)
    
    var (
        file *os.File
        fileOffset int64
        bytesRead int64
    )
    
    if file, err = openFile(fileName); err!=nil {
        return 0, err
    }
    defer closeFile(file)
    
    fileOffset = 0
    numMapFiles = 0
    chunk := make([]byte, chunkSize+utf8.UTFMax-1) // Adds 3 extra bytes to properly handle trailing multi-bytes runes in the case they occur;
    chunk, bytesRead = readChunk(file, fileOffset, chunk)
    for bytesRead>0 { // Reads the file in chunks;
        maxOffset := findChunkMaxOffset(chunk, chunkSize)
        if err = createFile(mapFileName(numMapFiles), chunk[:maxOffset]); err!=nil {
            return numMapFiles, err
        }
        fileOffset += maxOffset
        numMapFiles++
        chunk, bytesRead = readChunk(file, fileOffset, chunk)
    }
    
    return numMapFiles, nil
}

func openFile(fileName string) (file *os.File, err error) {
    if file, err = os.Open(fileName); err != nil {
        log.Fatal(err)
        return nil, err
    }
    if _, err = file.Stat(); err != nil {
        log.Fatal(err)
        return nil, err
    }
    return file, nil
}

func readChunk(file *os.File, fileOffset int64, chunk []byte) (trimmedChunk []byte, bytesRead int64) {
    bytes, err := file.ReadAt(chunk, fileOffset)
    if err!=nil && err!=io.EOF {
        log.Fatal(err)
        return chunk, 0
    }
    if bytes<len(chunk) {
        return chunk[:bytes], int64(bytes) // (*) "Trimming" chunk to bytesRead; this also implies this is a last chunk;
    } else {
        return chunk, int64(bytes)
    }
}

func findChunkMaxOffset(chunk []byte, chunkSize int64) (chunkMaxOffset int64) {
    chunkLength := int64(len(chunk))
    if chunkLength<chunkSize { // Last chunk; see (*) above;
        return chunkLength
    }
    for chunkOffset:=chunkSize-1; chunkOffset>0; {
        char, charStartOffset, charBytes := findPreviousChar(chunk, chunkOffset, false)
        if char==0 { // Error: could not find previous char!
            return 0
        } else if charStartOffset+charBytes<=chunkSize { // Current char respects chunkSize constraint!
            if isWordChar(char) {
                nextChar, _ := utf8.DecodeRune(chunk[charStartOffset+charBytes:])
                if(isWordChar(nextChar)) { // Warning: word split!
                    char, charStartOffset, charBytes = findPreviousChar(chunk, charStartOffset-1, true) // Search for the previous "non-word" char;
                    if char==0 { // Error: could not find previous char!
                        return 0
                    }
                }
            }
            return charStartOffset + charBytes; // Found max offset!
        } else { // Keep searching;
            chunkOffset = charStartOffset-1 // Current char doesn't respect chunkSize constraint: update chunkOffset for next iteration;      
        }
    }
    return 0
}

func findPreviousChar(chunk []byte, chunkOffset int64, searchForNonWordCharsOnly bool) (char rune, charStartOffset int64, charBytes int64) {
    var bytes int
    for charStartOffset = chunkOffset; charStartOffset>0; charStartOffset-- {
        if utf8.RuneStart(chunk[charStartOffset]) {
            char, bytes = utf8.DecodeRune(chunk[charStartOffset:])
            keepSearching := isWordChar(char) && searchForNonWordCharsOnly
            if !keepSearching { // Found char;
                return char, charStartOffset, int64(bytes)
            }
        }
    }
    return 0, 0, 0
}

func isWordChar(char rune) (isWordChar bool) {
    return unicode.IsLetter(char) || unicode.IsNumber(char)
}

func closeFile(file *os.File) {
    file.Close()
}

func createFile(fileName string, data []byte) (err error) {
    var file *os.File
    if file, err = os.Create(fileName); err != nil {
        log.Fatal(err)
		return err
	}
    defer closeFile(file)
    if _, err = file.Write(data); err != nil {
        log.Fatal(err)
		return err
	}
    return nil
}

func mapFileName(id int) string {
	return filepath.Join(MAP_PATH, fmt.Sprintf("map-%v", id))
}

func resultFileName(id int) string {
	return filepath.Join(RESULT_PATH, fmt.Sprintf("result-%v", id))
}
