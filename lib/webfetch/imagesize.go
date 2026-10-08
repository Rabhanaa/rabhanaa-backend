package webfetch

import (
	"bytes"
	"encoding/binary"
	"image"

	// Decoders registered for image.DecodeConfig.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// ImageSize reads an image's dimensions from its header without decoding it.
// ok is false for formats it cannot read, in which case callers should not
// assume the image is small.
func ImageSize(data []byte, contentType string) (width, height int, ok bool) {
	if contentType == "image/webp" {
		return webpSize(data)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// webpSize handles the three WebP layouts. The standard library has no WebP
// decoder, and most news sites now serve their photos as WebP.
func webpSize(b []byte) (int, int, bool) {
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(b[12:16]) {
	case "VP8X": // extended: 24-bit width-1 and height-1
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return w + 1, h + 1, true
	case "VP8 ": // lossy: 14-bit sizes after the frame start code
		if b[23] != 0x9d || b[24] != 0x01 || b[25] != 0x2a {
			return 0, 0, false
		}
		w := int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff)
		return w, h, true
	case "VP8L": // lossless: two 14-bit fields packed after the signature
		if b[20] != 0x2f {
			return 0, 0, false
		}
		bits := binary.LittleEndian.Uint32(b[21:25])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1, true
	}
	return 0, 0, false
}
