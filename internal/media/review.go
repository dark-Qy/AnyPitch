package media

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
)

// ReviewJPEG validates and normalizes the exact image sent to WeChat and stored
// after approval. No EXIF or unvalidated input is made downloadable.
func ReviewJPEG(source io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(source, MaxUploadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > MaxUploadBytes {
		return nil, ErrInvalidPhoto
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !validFormat(format) || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, ErrInvalidPhoto
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidPhoto
	}
	var out bytes.Buffer
	err = jpeg.Encode(&out, resize(flatten(img), FullImageEdge), &jpeg.Options{Quality: 85})
	if err != nil {
		return nil, err
	}
	if out.Len() > MaxUploadBytes {
		return nil, ErrInvalidPhoto
	}
	return out.Bytes(), nil
}
