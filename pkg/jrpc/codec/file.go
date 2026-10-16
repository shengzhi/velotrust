package codec

import (
	"bytes"
	"io"
	"mime"
	"net/http"
)

type FileCodec struct {
}

func (fc FileCodec) WriteResponse(w http.ResponseWriter, b []byte, err error) error {
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return nil
	}
	r := bytes.NewReader(b)
	w.Header().Set("Content-Type", fc.detectContentType(r))
	w.WriteHeader(200)
	io.Copy(w, r)
	return nil
}

func (fc FileCodec) detectContentType(r io.ReadSeeker) string {
	if CheckFileType(r, PNG) {
		return mime.TypeByExtension(".png")
	}
	if CheckFileType(r, JPG) {
		return mime.TypeByExtension(".jpeg")
	}
	if CheckFileType(r, GIF) {
		return mime.TypeByExtension(".gif")
	}
	if CheckFileType(r, PDF) {
		return mime.TypeByExtension(".pdf")
	}
	if CheckFileType(r, GZIP) {
		return mime.TypeByExtension(".gz")
	}
	if CheckFileType(r, RAR) {
		return mime.TypeByExtension(".rar")
	}
	if CheckFileType(r, DOC) {
		return mime.TypeByExtension(".doc")
	}
	if CheckFileType(r, DOCX) {
		return mime.TypeByExtension(".docx")
	}
	return "application/octet-stream"
}
