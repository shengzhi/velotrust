package codec

import (
	"bytes"
	"io"
)

type FileMagicNumber []byte

var (
	PDF   FileMagicNumber = []byte{0x25, 0x50, 0x44, 0x46}
	RAR   FileMagicNumber = []byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x00}
	GZIP  FileMagicNumber = []byte{0x1F, 0x8B, 0x08}
	ZIP_0 FileMagicNumber = []byte{0x50, 0x4B, 0x03, 0x04}
	ZIP_1 FileMagicNumber = []byte{0x50, 0x4B, 0x05, 0x06}
	ZIP_2 FileMagicNumber = []byte{0x50, 0x4B, 0x07, 0x08}
	WEBP  FileMagicNumber = []byte{0x52, 0x49, 0x46, 0x46}
	PNG   FileMagicNumber = []byte{0x89, 0x50, 0x4e, 0x47}
	JPG   FileMagicNumber = []byte{0xff, 0xd8, 0xff, 0xe0}
	GIF   FileMagicNumber = []byte{0x47, 0x49, 0x46, 0x38}
	DOC   FileMagicNumber = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	DOCX  FileMagicNumber = []byte{0x50, 0x4B, 0x03, 0x04}
)

func CheckFileType(r io.ReadSeeker, magic FileMagicNumber) bool {
	return genericCompareBuffer(r, magic)
}

func readUntil(l int, r io.ReadSeeker) ([]byte, error) {
	buff := make([]byte, l)

	_, err := r.Read(buff)
	if err != nil {
		return nil, err
	}

	r.Seek(0, io.SeekStart)

	return buff, nil
}

func checkBuffer(r io.ReadSeeker, t []byte) ([]byte, error) {
	// 根据提供参数获取长度
	l := len(t)

	buff, err := readUntil(l, r)
	if err != nil {
		return make([]byte, 0), err
	}

	return buff, nil
}
func genericCompareBuffer(r io.ReadSeeker, t []byte) bool {
	buff, err := checkBuffer(r, t)
	if err != nil {
		return false
	}

	valid := bytes.Compare(t, buff)
	return valid == 0
}
