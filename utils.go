package vdfloc

import (
	"fmt"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/encoding/unicode/utf32"
	"golang.org/x/text/transform"
)

// byte order mark bytes
var (
	Utf8bom    = []byte{0xEF, 0xBB, 0xBF}
	Utf16LEbom = []byte{0xFF, 0xFE}
	Utf16BEbom = []byte{0xFE, 0xFF}
	Utf32LEbom = []byte{0xFF, 0xFE, 0x00, 0x00}
	Utf32BEbom = []byte{0x00, 0x00, 0xFE, 0xFF}
)

const utf8ProbeLen = 4 * 32 * 1024 // probe length: if this length utf8 then the rest of the file is utf8

// https://ompp.sourceforge.io/src/go.openmpp.org/ompp/helper/utf8.go Utf8Reader
// UTFReader returns a reader to transform file content to utf-8.
//
// 		Keep the original source mechanics. Modifications are:
//			- Added 1 output param: input file encoding detected.
//			- Falls back to utf8 (not OS dependent)
// The output io.Reader skips the BOM
// If file starts with BOM (utf-8 utf-16LE utf-16BE utf-32LE utf-32BE) then BOM is used.
// If no BOM and encodingName is "" empty then file content probed to see is it already utf-8.
// If encodingName explicitly specified then it is used to convert file content to string.
// If none of above then assume default encoding: "windows-1252" on Windows and "utf-8" on Linux.
func UTFReader(f *os.File, encodingName string) (r io.Reader, encodingFound string, err error) {

	// validate parameters
	if f == nil {
		return nil, encodingFound, errors.New("invalid (nil) source file")
	}

	// detect BOM
	bom := make([]byte, utf8.UTFMax)

	nBom, err := f.Read(bom)
	if err != nil {
		if nBom == 0 && err == io.EOF { // empty file: retrun source file as is
			return f, encodingFound, nil
		}
		return nil, encodingFound, errors.New("file read error: " + err.Error())
	}

	// if utf-8 BOM then skip it and return source file
	if nBom >= len(Utf8bom) && bom[0] == Utf8bom[0] && bom[1] == Utf8bom[1] && bom[2] == Utf8bom[2] {
		if _, err := f.Seek(int64(len(Utf8bom)), 0); err != nil {
			return nil, encodingFound, errors.New("file seek error: " + err.Error())
		}
		return f, "UTF8BOM", nil
	}

	// move back to the file begining to use BOM, if present
	if _, err := f.Seek(0, 0); err != nil {
		return nil, encodingFound, errors.New("file seek error (moving back) " + err.Error())
	}

	// ambiguous utf-16LE and utf32-LE detection: assume utf-32LE because 00 00 is very unlikely in text file
	if nBom >= len(Utf32LEbom) && bom[0] == Utf32LEbom[0] && bom[1] == Utf32LEbom[1] && bom[2] == Utf32LEbom[2] && bom[3] == Utf32LEbom[3] {
		return transform.NewReader(f, utf32.UTF32(utf32.LittleEndian, utf32.UseBOM).NewDecoder()), "UTF32LE", nil
	}
	if nBom >= len(Utf32BEbom) && bom[0] == Utf32BEbom[0] && bom[1] == Utf32BEbom[1] && bom[2] == Utf32BEbom[2] && bom[3] == Utf32BEbom[3] {
		return transform.NewReader(f, utf32.UTF32(utf32.BigEndian, utf32.UseBOM).NewDecoder()), "UTF32BE", nil
	}
	if nBom >= len(Utf16LEbom) && bom[0] == Utf16LEbom[0] && bom[1] == Utf16LEbom[1] {
		return transform.NewReader(f, unicode.BOMOverride(encoding.Nop.NewDecoder())), "UTF16LE", nil
	}
	if nBom >= len(Utf16BEbom) && bom[0] == Utf16BEbom[0] && bom[1] == Utf16BEbom[1] {
		return transform.NewReader(f, unicode.BOMOverride(encoding.Nop.NewDecoder())), "UTF16BE", nil
	}
	// no BOM detected

	// encoding not specified then probe file to check is it utf-8
	if encodingName == "" {

		// read probe bytes from the file
		buf := make([]byte, utf8ProbeLen)
		nProbe, err := f.Read(buf)
		if err != nil {
			if nProbe == 0 && err == io.EOF { // empty file: retrun source file as is
				return f, "UTF32LE", nil
			}
			return nil, encodingFound, errors.New("file read error: " + err.Error())
		}

		// check if all runes are utf-8
		nPos := 0
		for nPos < nProbe {
			r, n := utf8.DecodeRune(buf)
			if n <= 0 || r == utf8.RuneError { // if eof or not utf-8 rune
				break
			}
			nPos += n
			buf = buf[n:]
		}

		// move back to the file begining
		if _, err := f.Seek(0, 0); err != nil {
			return nil, encodingFound, errors.New("file seek error: " + err.Error())
		}

		// file is utf-8 if:
		// all runes are utf-8 and file size less than max probe size or file size excceeds probe size
		if nPos >= nProbe || nPos >= utf8ProbeLen-utf8.UTFMax {
			return f, "UTF8", nil // utf-8 file: return source file reader
		}
	}

	// if encoding is not explicitly specified then use UTF8
	if encodingName == "" {
		//if runtime.GOOS == "windows" {
		//	encodingName = "windows-1252"
		//} else {
		encodingName = "UTF8"
		//}
	}

	// get encoding by name
	enc, err := htmlindex.Get(encodingName)
	encodingFound = encodingName
	if err != nil {
		return nil, encodingFound, errors.New("invalid encoding: " + encodingName + " " + err.Error())
	}

	return transform.NewReader(f, unicode.BOMOverride(enc.NewDecoder())), encodingFound, nil
}

// UTF8Conv()
// Convert a UTF8 buffer to UTF16BE or LE
//	encodingName can be UTF16LE, UTF16BE, UTF8BOM, UTF8
// 	if encoding name is UTF8 or UTF8BOM returns buf
//
func UTF8Conv(buf []byte, encodingName string) (out []byte, err error) {

	var enc encoding.Encoding
	var bom []byte
	
	switch strings.ToLower(encodingName) {
	case "utf8":
		return buf, nil
	case "utf8bom":
		bom = []byte{0xEF, 0xBB, 0xBF}  // printout a BOM
		return buf, nil
	case "utf16le":
		enc = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM )
	case "utf16be":
		enc = unicode.UTF16(unicode.BigEndian, unicode.UseBOM )
	}

	var utfEncoder *encoding.Encoder
	utfEncoder = enc.NewEncoder()
	out, err = utfEncoder.Bytes(buf)
	
	return append(bom,out...), nil
}

type UTF8Enc struct {
	encoding    string		// "UTF8", "UTF8BOM", etc.
	utfEncoder	*encoding.Encoder
	f           *os.File
	ioName		string      // file, stdout, etc.
}
// Create a new instance
// - In: File (nil for stdout) and encoding
// - Returns instance and error code
func NewUTFConvWriter(f *os.File, encodingName string) (u *UTF8Enc, err error) {

	u = &UTF8Enc{} // Create instance

	if f == nil {
		// Stdout
		u.f = os.Stdout
	}
	u.f = f
	u.ioName = f.Name()

	var enc encoding.Encoding

	// fmt.Printf("enc=%s, outname=%s\n",encodingName, u.ioName)

	switch strings.ToLower(encodingName) {
	case "utf8":
	case "utf8bom":
		f.Write([]byte{0xEF, 0xBB, 0xBF})		// printout a BOM
	case "utf16le":
		f.Write([]byte{0xFF, 0xFE})  			// printout a BOM
		enc = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM )
	case "utf16be":
		f.Write([]byte{0xFE, 0xFF})			// printout a BOM
		enc = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM )
	default:
	}

	if enc != nil {
		u.utfEncoder = enc.NewEncoder()
	}

	return u, nil
}


// UTF8ConvWriter()
// Convert a UTF8 buffer to UTF16BE or LE
//	encodingName can be UTF16LE, UTF16BE, UTF8BOM, UTF8
// 	if encoding name is UTF8 or UTF8BOM skip the convertion
// Returns the number of bytes writen
//
func (u *UTF8Enc) Write(buf []byte) (n int, err error) {
	var out []byte
	if u.utfEncoder != nil {
		out, err = u.utfEncoder.Bytes(buf)
		if err != nil {
			return 0, fmt.Errorf("Unable to convert %v - %v", buf, err)
		}
	} else {
		out = buf
	}
	n, err = u.f.Write(out)
	if err != nil {
		return 0, fmt.Errorf("Unable to write: %v", err)
	}
	return n, nil	
}

// Close()
// 
func (u *UTF8Enc) Close() (err error) {
	return u.f.Close()
}
