package kademila

/*
#cgo LDFLAGS: -lreadline -lhistory
#include <stdio.h>
#include <stdlib.h>
#include <readline/readline.h>
#include <readline/history.h>
size_t _GoStringLen(_GoString_ s);
const char *_GoStringPtr(_GoString_ s);
*/
import "C"
import (
	"bufio"
	"fmt"
	"io"
	"os"
	"unsafe"
)

var count = 1

func GNUReadLine(prompt string) (string, bool) {
	p := fmt.Sprintf("[%d] %s", count, prompt)
	s := C.readline(C._GoStringPtr(p))
	if s == nil {
		// Ctrl-D, EOF
		return "", false
	}
	defer C.free(unsafe.Pointer(s))
	p = C.GoString(s)
	if len(p) > 0 {
		count++
	}
	return p, true
}

func GNUAddHistory(line string) {
	if len(line) == 0 {
		return
	}
	s := C._GoStringPtr(line)
	C.add_history(s)
}

var scanner = bufio.NewScanner(os.Stdin)

func GoReadLine(prompt string) (string, bool) {
	io.WriteString(os.Stdout, prompt)
	if !scanner.Scan() {
		return "", false
	}
	return scanner.Text(), true
}
