package tags

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"
)

// MPEG audio bitrate tables in kbps, indexed by [mpegGroup][layerIndex][bitrateIndex].
// mpegGroup: 0 = MPEG1, 1 = MPEG2/2.5. layerIndex: 0 = Layer1, 1 = Layer2, 2 = Layer3.
// Index 0 (free) and 15 (invalid) are 0 and rejected by the parser.
var mp3BitrateTable = [2][3][16]int{
	{ // MPEG1
		{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448, 0}, // Layer1
		{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0},    // Layer2
		{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},     // Layer3
	},
	{ // MPEG2 / MPEG2.5
		{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0}, // Layer1
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},      // Layer2
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},      // Layer3
	},
}

// mp3SampleRates is indexed by MPEG version (1, 2, 25) then sample-rate index (0-2).
var mp3SampleRates = map[int][3]int{
	1:  {44100, 48000, 32000},
	2:  {22050, 24000, 16000},
	25: {11025, 12000, 8000},
}

// mp3FrameHeader holds the fields of an MPEG audio frame header relevant to duration.
type mp3FrameHeader struct {
	version     int  // 1, 2, or 25 (MPEG2.5)
	layer       int  // 1, 2, or 3
	bitrate     int  // kbps
	sampleRate  int  // Hz
	channelMode int  // 0=stereo, 1=joint, 2=dual, 3=mono
	padding     bool // padding slot present in this frame
}

// parseMP3FrameHeader decodes a 4-byte MPEG audio frame header, returning ok=false
// for invalid/reserved combinations.
func parseMP3FrameHeader(b []byte) (mp3FrameHeader, bool) {
	var h mp3FrameHeader

	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return h, false
	}

	switch (b[1] >> 3) & 0x03 {
	case 0:
		h.version = 25
	case 2:
		h.version = 2
	case 3:
		h.version = 1
	default:
		return h, false // reserved
	}

	switch (b[1] >> 1) & 0x03 {
	case 1:
		h.layer = 3
	case 2:
		h.layer = 2
	case 3:
		h.layer = 1
	default:
		return h, false // reserved
	}

	bitrateIdx := (b[2] >> 4) & 0x0F
	if bitrateIdx == 0 || bitrateIdx == 15 {
		return h, false // free or invalid
	}

	group := 0
	if h.version != 1 {
		group = 1
	}

	h.bitrate = mp3BitrateTable[group][h.layer-1][bitrateIdx]
	if h.bitrate == 0 {
		return h, false
	}

	srIdx := (b[2] >> 2) & 0x03
	if srIdx == 3 {
		return h, false
	}

	rates, ok := mp3SampleRates[h.version]
	if !ok {
		return h, false
	}

	h.sampleRate = rates[srIdx]
	h.channelMode = int((b[3] >> 6) & 0x03)
	h.padding = (b[2]>>1)&0x01 == 1

	return h, true
}

// frameLength returns the size in bytes of this MPEG audio frame, used to step
// from one frame header to the next when counting frames in a VBR file.
func (h mp3FrameHeader) frameLength() int {
	if h.sampleRate == 0 || h.bitrate == 0 {
		return 0
	}

	br := h.bitrate * 1000

	pad := 0
	if h.padding {
		pad = 1
	}

	if h.layer == 1 {
		return (12*br/h.sampleRate + pad) * 4
	}

	// Layer II is always 144; Layer III is 144 for MPEG1 and 72 for MPEG2/2.5.
	coeff := 144
	if h.version != 1 && h.layer == 3 {
		coeff = 72
	}

	return coeff*br/h.sampleRate + pad
}

// samplesPerFrame returns the number of PCM samples encoded per frame.
func (h mp3FrameHeader) samplesPerFrame() int {
	switch h.layer {
	case 1:
		return 384
	case 2:
		return 1152
	case 3:
		if h.version == 1 {
			return 1152
		}

		return 576
	}

	return 0
}

// sideInfoSize returns the Layer III side-information size, used to locate a
// Xing/Info header within the first frame.
func (h mp3FrameHeader) sideInfoSize() int {
	mono := h.channelMode == 3

	if h.version == 1 {
		if mono {
			return 17
		}

		return 32
	}

	if mono {
		return 9
	}

	return 17
}

// mp3Duration computes the playback duration of an MP3 without spawning ffprobe.
// It skips any ID3v2 tag, parses the first MPEG audio frame, and uses a Xing/Info
// or VBRI frame count for an exact VBR duration, falling back to a CBR estimate
// from the audio byte length and the frame's bitrate.
func mp3Duration(path string) (time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}

	fileSize := fi.Size()

	// Skip an ID3v2 tag if present (10-byte header + syncsafe size [+ footer]).
	var hdr [10]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return 0, err
	}

	var audioStart int64
	if string(hdr[0:3]) == "ID3" {
		size := int64(hdr[6]&0x7f)<<21 | int64(hdr[7]&0x7f)<<14 |
			int64(hdr[8]&0x7f)<<7 | int64(hdr[9]&0x7f)
		if hdr[5]&0x10 != 0 { // footer present
			size += 10
		}

		audioStart = 10 + size
	}

	if _, err := f.Seek(audioStart, io.SeekStart); err != nil {
		return 0, err
	}

	buf := make([]byte, 8192)

	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return 0, err
	}

	buf = buf[:n]

	// Locate the first valid frame header.
	frameOffset := -1

	var fh mp3FrameHeader

	for i := 0; i+4 <= len(buf); i++ {
		if buf[i] != 0xFF || buf[i+1]&0xE0 != 0xE0 {
			continue
		}

		if h, ok := parseMP3FrameHeader(buf[i : i+4]); ok {
			fh = h
			frameOffset = i

			break
		}
	}

	if frameOffset < 0 {
		return 0, errors.New("no MPEG audio frame found")
	}

	samplesPerFrame := fh.samplesPerFrame()
	if samplesPerFrame == 0 || fh.sampleRate == 0 {
		return 0, errors.New("invalid frame parameters")
	}

	// Best: an exact frame count from a Xing/Info (Layer III) or VBRI header,
	// which the vast majority of VBR/ABR (and many CBR) files carry.
	if frames, ok := mp3VBRFrameCount(buf, frameOffset, fh); ok && frames > 0 {
		return framesToDuration(int64(frames), samplesPerFrame, fh.sampleRate), nil
	}

	// No VBR header: walk the actual frames. This is exact for both CBR and
	// headerless VBR — each frame holds a fixed sample count regardless of its
	// bitrate, so a first-frame-bitrate estimate would be wrong for VBR.
	if frames := mp3CountFrames(f, audioStart+int64(frameOffset)); frames > 0 {
		return framesToDuration(frames, samplesPerFrame, fh.sampleRate), nil
	}

	// Last resort: estimate from byte length and the first frame's bitrate
	// (accurate only for CBR).
	audioBytes := fileSize - (audioStart + int64(frameOffset))
	if audioBytes > 0 && fh.bitrate > 0 {
		secs := float64(audioBytes) * 8 / float64(fh.bitrate*1000)
		return time.Duration(secs * float64(time.Second)), nil
	}

	return 0, errors.New("could not determine duration")
}

// framesToDuration converts a frame count to a duration.
func framesToDuration(frames int64, samplesPerFrame, sampleRate int) time.Duration {
	secs := float64(frames) * float64(samplesPerFrame) / float64(sampleRate)
	return time.Duration(secs * float64(time.Second))
}

// mp3CountFrames walks every MPEG audio frame from start to EOF and returns the
// count, stepping by each frame's own length so VBR is handled correctly. It
// stops at the first non-frame bytes (e.g. an ID3v1 trailer or padding).
func mp3CountFrames(f *os.File, start int64) int64 {
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return 0
	}

	br := bufio.NewReaderSize(f, 64*1024)

	var (
		hdr   [4]byte
		count int64
	)

	for {
		if _, err := io.ReadFull(br, hdr[:]); err != nil {
			break
		}

		h, ok := parseMP3FrameHeader(hdr[:])
		if !ok {
			break
		}

		flen := h.frameLength()
		if flen < 4 {
			break
		}

		count++

		if _, err := br.Discard(flen - 4); err != nil {
			break
		}
	}

	return count
}

// mp3VBRFrameCount returns the total frame count from a Xing/Info or VBRI header
// in the first frame, if present.
func mp3VBRFrameCount(buf []byte, frameOffset int, fh mp3FrameHeader) (uint32, bool) {
	// Xing/Info sits after the Layer III side information.
	if fh.layer == 3 {
		xing := frameOffset + 4 + fh.sideInfoSize()
		if xing+12 <= len(buf) {
			tag := string(buf[xing : xing+4])
			if tag == "Xing" || tag == "Info" {
				flags := binary.BigEndian.Uint32(buf[xing+4 : xing+8])
				if flags&0x0001 != 0 { // frames field present
					return binary.BigEndian.Uint32(buf[xing+8 : xing+12]), true
				}
			}
		}
	}

	// VBRI (Fraunhofer) is at a fixed offset of 32 bytes after the frame header.
	vbri := frameOffset + 4 + 32
	if vbri+18 <= len(buf) && string(buf[vbri:vbri+4]) == "VBRI" {
		return binary.BigEndian.Uint32(buf[vbri+14 : vbri+18]), true
	}

	return 0, false
}
