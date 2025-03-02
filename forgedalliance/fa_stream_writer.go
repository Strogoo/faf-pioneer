package forgedalliance

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
)

// FaStreamWriter writes messages in a format compatible with the FA stream protocol.
type FaStreamWriter struct {
	w  *bufio.Writer
	mu sync.Mutex
}

// NewFaStreamWriter creates a new writer for the given output stream.
func NewFaStreamWriter(w *bufio.Writer) *FaStreamWriter {
	log.Println("FaStreamWriter opened")
	return &FaStreamWriter{
		w: w,
	}
}

// writeString writes a string with its length prefix.
func (w *FaStreamWriter) writeString(s string) error {
	// Write the length of the string
	if err := binary.Write(w.w, binary.LittleEndian, int32(len(s))); err != nil {
		return err
	}
	// Write the string bytes
	_, err := w.w.Write([]byte(s))
	return err
}

// writeArgs writes a list of arguments with their types.
func (w *FaStreamWriter) writeArgs(args []interface{}) error {
	// Write number of arguments
	if err := binary.Write(w.w, binary.LittleEndian, int32(len(args))); err != nil {
		return err
	}

	// Iterate over each argument and write it
	for index, arg := range args {
		switch v := arg.(type) {
		case int:
			w.w.WriteByte(FaStreamConstants.FieldTypes.INT)
			binary.Write(w.w, binary.LittleEndian, int32(v))
		case uint:
			w.w.WriteByte(FaStreamConstants.FieldTypes.INT)
			binary.Write(w.w, binary.LittleEndian, int32(v))
		case uint16:
			w.w.WriteByte(FaStreamConstants.FieldTypes.INT)
			binary.Write(w.w, binary.LittleEndian, int32(v))
		case uint32:
			w.w.WriteByte(FaStreamConstants.FieldTypes.INT)
			binary.Write(w.w, binary.LittleEndian, v)
		case int32:
			w.w.WriteByte(FaStreamConstants.FieldTypes.INT)
			binary.Write(w.w, binary.LittleEndian, v)
		case string:
			w.w.WriteByte(FaStreamConstants.FieldTypes.STRING)
			if err := w.writeString(v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Unexpected type %T in arguments (index %d)\n", v, index)
		}
	}
	return nil
}

// WriteMessage writes a GpgnetMessage to the output stream.
func (w *FaStreamWriter) WriteMessage(gpgnetMessage GpgMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log.Printf("Writing message: %v\n", gpgnetMessage)

	if err := w.writeString(gpgnetMessage.GetCommand()); err != nil {
		return err
	}
	if err := w.writeArgs(gpgnetMessage.GetArgs()); err != nil {
		return err
	}

	return w.w.Flush()
}

// Close closes the FaStreamWriter.
func (w *FaStreamWriter) Close() error {
	log.Println("Closing FaStreamWriter")
	return w.w.Flush()
}

// FaStreamConstants defines constants for field types.
var FaStreamConstants = struct {
	FieldTypes struct {
		INT    byte
		STRING byte
	}
}{
	FieldTypes: struct {
		INT    byte
		STRING byte
	}{
		INT:    0x01,
		STRING: 0x02,
	},
}
