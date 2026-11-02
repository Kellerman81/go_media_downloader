package tags

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// OGGHandler handles reading and writing Vorbis Comment tags for OGG Vorbis files.
type OGGHandler struct{}

// NewOGGHandler creates a new OGG Vorbis tag handler.
func NewOGGHandler() *OGGHandler {
	return &OGGHandler{}
}

// SupportedFormats returns the file extensions supported by this handler.
// Covers both Ogg Vorbis (.ogg/.oga) and Ogg Opus (.opus), which share the same
// page framing and Vorbis-comment metadata structure.
func (*OGGHandler) SupportedFormats() []string {
	return []string{".ogg", ".oga", ".opus"}
}

// commentHeaderPayload reports the offset of the Vorbis-comment payload (vendor
// string onward) within an Ogg comment-header packet, for both Vorbis ("\x03vorbis")
// and Opus ("OpusTags") streams. ok is false if data is not a comment header.
func commentHeaderPayload(data []byte) (offset int, ok bool) {
	if len(data) >= 8 && string(data[0:8]) == "OpusTags" {
		return 8, true
	}

	if len(data) > 7 && data[0] == 0x03 && string(data[1:7]) == "vorbis" {
		return 7, true
	}

	return 0, false
}

// parseIDHeader fills audio properties from an Ogg identification header for both
// Vorbis ("\x01vorbis") and Opus ("OpusHead") streams.
func parseIDHeader(data []byte, tags *AudioTags) {
	if len(data) >= 19 && string(data[0:8]) == "OpusHead" {
		tags.Channels = int(data[9])
		// Opus stores the original input sample rate; decoding is always 48 kHz.
		tags.SampleRate = int(binary.LittleEndian.Uint32(data[12:16]))

		return
	}

	if len(data) > 29 && data[0] == 0x01 && string(data[1:7]) == "vorbis" {
		tags.Channels = int(data[11])
		tags.SampleRate = int(binary.LittleEndian.Uint32(data[12:16]))
		tags.Bitrate = int(binary.LittleEndian.Uint32(data[20:24])) / 1000 // nominal, kbps
	}
}

// oggPage represents an OGG page header and data.
type oggPage struct {
	Version         uint8
	HeaderType      uint8
	GranulePos      int64
	BitstreamSerial uint32
	PageSequence    uint32
	Checksum        uint32
	PageSegments    uint8
	SegmentTable    []uint8
	Data            []byte
}

// oggMaxHeaderBytes bounds how much header-packet data (mostly embedded cover
// art) is buffered, guarding against corrupt or hostile files.
const oggMaxHeaderBytes = 64 << 20

// errOggUnsupportedCodec marks Ogg streams that are neither Vorbis nor Opus
// (e.g. Ogg FLAC, Speex), which carry no comment header this package parses.
var errOggUnsupportedCodec = errors.New("unsupported OGG codec (only Vorbis and Opus are supported)")

// oggHeaders holds the codec header packets of an Ogg Vorbis/Opus stream.
type oggHeaders struct {
	firstPage []byte   // raw BOS page; carries only the identification header
	packets   [][]byte // id + comment (Opus), id + comment + setup (Vorbis)
	serial    uint32
	firstSeq  uint32
	version   uint8
	opus      bool
	pages     int  // pages carrying header packets, including the BOS page
	eos       bool // a header page after the BOS page had the EOS flag set
}

// readOggHeaders reads and reassembles the header packets of the first logical
// stream, consuming exactly the pages that carry them. Packets are rebuilt from
// the lacing values across page boundaries, so header packets larger than a
// page (embedded cover art) are handled. With strict set, layouts the writer
// cannot rewrite safely (interleaved logical streams, audio data sharing the
// last header page) are rejected; otherwise pages of other streams are skipped.
func (h *OGGHandler) readOggHeaders(r io.Reader, strict bool) (*oggHeaders, error) {
	first, raw, err := h.readPageRaw(r)
	if err != nil {
		return nil, err
	}

	if first.HeaderType&0x02 == 0 {
		return nil, errors.New("first OGG page is not a beginning-of-stream page")
	}

	// Both Vorbis and Opus require the identification header alone on the BOS page.
	nseg := len(first.SegmentTable)
	if nseg == 0 || first.SegmentTable[nseg-1] == 255 {
		return nil, errors.New("OGG identification header does not end on the first page")
	}

	for _, l := range first.SegmentTable[:nseg-1] {
		if l < 255 {
			return nil, errors.New("unexpected extra packet on the first OGG page")
		}
	}

	hd := &oggHeaders{
		firstPage: raw,
		packets:   [][]byte{first.Data},
		serial:    first.BitstreamSerial,
		firstSeq:  first.PageSequence,
		version:   first.Version,
		pages:     1,
	}

	var need int

	switch {
	case len(first.Data) >= 8 && string(first.Data[:8]) == "OpusHead":
		hd.opus = true
		need = 2
	case len(first.Data) >= 7 && first.Data[0] == 0x01 && string(first.Data[1:7]) == "vorbis":
		need = 3
	default:
		return nil, errOggUnsupportedCodec
	}

	var cur []byte

	total := len(first.Data)

	for len(hd.packets) < need {
		page, err := h.readPage(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}

			return nil, fmt.Errorf("reading OGG header pages: %w", err)
		}

		if page.BitstreamSerial != hd.serial {
			if strict {
				return nil, errors.New("interleaved OGG logical streams are not supported")
			}

			continue
		}

		hd.pages++

		if page.HeaderType&0x04 != 0 {
			hd.eos = true
		}

		pos := 0

		for i, l := range page.SegmentTable {
			cur = append(cur, page.Data[pos:pos+int(l)]...)
			pos += int(l)

			total += int(l)
			if total > oggMaxHeaderBytes {
				return nil, errors.New("OGG header packets too large")
			}

			if l == 255 {
				continue
			}

			hd.packets = append(hd.packets, cur)
			cur = nil

			if len(hd.packets) == need {
				if strict && i != len(page.SegmentTable)-1 {
					return nil, errors.New("audio data shares the last OGG header page")
				}

				break
			}
		}
	}

	return hd, nil
}

// ReadTags reads Vorbis Comment tags from an OGG Vorbis / Opus file.
func (h *OGGHandler) ReadTags(filepath string) (*AudioTags, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}
	defer file.Close()

	hd, err := h.readOggHeaders(bufio.NewReaderSize(file, 64<<10), false)
	if err != nil {
		if errors.Is(err, errOggUnsupportedCodec) {
			return &AudioTags{}, nil
		}

		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}

	tags := &AudioTags{}

	parseIDHeader(hd.packets[0], tags)

	if offset, ok := commentHeaderPayload(hd.packets[1]); ok {
		h.parseVorbisCommentPacket(hd.packets[1][offset:], tags)
	}

	return tags, nil
}

// ReadTagsWithCover reads Vorbis Comment tags including embedded cover art.
// ReadTags (below) already parses METADATA_BLOCK_PICTURE/COVERART
// unconditionally whenever present - unlike e.g. MP3, which skips cover
// frames on its normal ReadTags path for speed - so this is a direct alias,
// added purely to satisfy the CoverTagReader interface. Without it,
// Manager.ReadCoverData's type assertion on *OGGHandler failed and always
// returned (nil, ""), so WriteTags on an OGG/Opus file with no explicit new
// cover silently dropped any cover art the file already had.
func (h *OGGHandler) ReadTagsWithCover(filepath string) (*AudioTags, error) {
	return h.ReadTags(filepath)
}

// readPage reads a single OGG page from the file.
func (*OGGHandler) readPage(r io.Reader) (*oggPage, error) {
	// Read capture pattern
	capture := make([]byte, 4)
	if _, err := io.ReadFull(r, capture); err != nil {
		return nil, err
	}

	if string(capture) != "OggS" {
		return nil, errors.New("invalid OGG page: bad capture pattern")
	}

	page := &oggPage{}

	// Read header
	header := make([]byte, 23)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	page.Version = header[0]
	page.HeaderType = header[1]
	page.GranulePos = int64(
		binary.LittleEndian.Uint64(header[2:10]),
	)
	page.BitstreamSerial = binary.LittleEndian.Uint32(header[10:14])
	page.PageSequence = binary.LittleEndian.Uint32(header[14:18])
	page.Checksum = binary.LittleEndian.Uint32(header[18:22])
	page.PageSegments = header[22]

	// Read segment table
	page.SegmentTable = make([]uint8, page.PageSegments)
	if _, err := io.ReadFull(r, page.SegmentTable); err != nil {
		return nil, err
	}

	// Calculate total data size
	var dataSize int
	for i := range page.SegmentTable {
		dataSize += int(page.SegmentTable[i])
	}

	// Read page data
	page.Data = make([]byte, dataSize)
	if _, err := io.ReadFull(r, page.Data); err != nil {
		return nil, err
	}

	return page, nil
}

// parseVorbisCommentPacket parses the Vorbis comment packet data (vendor
// length onward). Comments parsed before a malformed entry are kept.
func (h *OGGHandler) parseVorbisCommentPacket(data []byte, tags *AudioTags) {
	_, comments, _, _ := parseVorbisCommentList(data)

	for _, comment := range comments {
		key, value, ok := strings.Cut(comment, "=")
		if !ok {
			continue
		}

		h.setTagValue(strings.ToUpper(key), value, tags)
	}
}

// setTagValue sets the appropriate tag field based on the key.
func (h *OGGHandler) setTagValue(key, value string, tags *AudioTags) {
	switch key {
	// Cover art (base64 encoded in METADATA_BLOCK_PICTURE)
	case "METADATA_BLOCK_PICTURE":
		h.parsePictureBlock(value, tags)
	case "COVERART": // Legacy cover art format
		if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
			tags.CoverData = decoded
			tags.CoverMIME = detectImageMIME(decoded)
		}

	default:
		applyVorbisComment(key, value, tags)
	}
}

// parsePictureBlock parses a FLAC-style METADATA_BLOCK_PICTURE. Lengths are
// checked in uint64 so crafted 32-bit length fields cannot wrap the bounds
// checks; malformed blocks yield no cover.
func (*OGGHandler) parsePictureBlock(encoded string, tags *AudioTags) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) < 32 {
		return
	}

	// Parse FLAC PICTURE block format
	pictureType := binary.BigEndian.Uint32(data[0:4])

	// Only use front cover (type 3) or if we don't have one yet
	if pictureType != 3 && tags.CoverData != nil {
		return
	}

	n := uint64(len(data))
	off := uint64(8)

	mimeLen := uint64(binary.BigEndian.Uint32(data[4:8]))
	if off+mimeLen+4 > n {
		return
	}

	mime := string(data[off : off+mimeLen])
	off += mimeLen

	// Skip description, then width, height, depth, colors (16 bytes).
	descLen := uint64(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4

	if off+descLen+16+4 > n {
		return
	}

	off += descLen + 16

	picLen := uint64(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4

	if off+picLen > n {
		return
	}

	tags.CoverData = data[off : off+picLen]
	tags.CoverMIME = mime
}

// detectImageMIME attempts to detect the MIME type from image data.
func detectImageMIME(data []byte) string {
	if len(data) < 4 {
		return "image/jpeg"
	}

	// Check magic bytes
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "image/jpeg"
	}

	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) {
		return "image/png"
	}

	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return "image/gif"
	}

	// WebP is a RIFF container: "RIFF"<4-byte size>"WEBP".
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) &&
		string(data[8:12]) == "WEBP" {
		return "image/webp"
	}

	return "image/jpeg" // Default
}

// WriteTags writes Vorbis Comment tags to an OGG Vorbis / Opus file.
//
// The header packets are reassembled across pages, only the comment packet is
// rebuilt (existing comments the new tags don't set are kept), and the header
// packets after the identification header (comment + setup for Vorbis,
// OpusTags for Opus) are re-paginated with correct lacing, granule positions,
// sequence numbers and CRCs. If the header page count changes, the sequence
// numbers (and CRCs) of all following pages of the stream are renumbered;
// otherwise the audio pages are copied verbatim. The result is streamed to a
// temp file next to the original, which then atomically replaces it.
func (h *OGGHandler) WriteTags(_ context.Context, filepath string, tags *AudioTags) error {
	in, err := os.Open(filepath)
	if err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}
	defer in.Close()

	mode := os.FileMode(0o644)
	if st, serr := in.Stat(); serr == nil {
		mode = st.Mode()
	}

	br := bufio.NewReaderSize(in, 64<<10)

	hd, err := h.readOggHeaders(br, true)
	if err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: "cannot rewrite OGG comments: " + err.Error()}
	}

	offset, ok := commentHeaderPayload(hd.packets[1])
	if !ok {
		return &ErrWriteFailed{Path: filepath, Reason: "could not find Vorbis comment header"}
	}

	headers := make([][]byte, 0, len(hd.packets)-1)
	headers = append(headers, h.buildCommentPacket(hd.packets[1], offset, hd.opus, tags))
	headers = append(headers, hd.packets[2:]...)

	pages := buildOggPages(headers, hd.version, hd.serial, hd.firstSeq+1, hd.eos)

	// Wraps modulo 2^32 when the header shrinks, which is what the
	// sequence-number arithmetic needs.
	delta := uint32(len(pages) - (hd.pages - 1)) //nolint:gosec // intentional wrap

	tmpPath := filepath + ".oggtmp"

	out, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	success := false
	defer func() {
		out.Close()

		if !success {
			os.Remove(tmpPath)
		}
	}()

	bw := bufio.NewWriterSize(out, 64<<10)

	if _, err := bw.Write(hd.firstPage); err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	for i := range pages {
		if _, err := bw.Write(pages[i]); err != nil {
			return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
		}
	}

	if delta == 0 {
		_, err = io.Copy(bw, br)
	} else {
		err = copyOggPagesRenumbered(bw, br, hd.serial, delta)
	}

	if err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	if err := bw.Flush(); err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	if err := out.Sync(); err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	// Close both files before replacing (required on Windows).
	if err := out.Close(); err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	in.Close()

	if err := os.Rename(tmpPath, filepath); err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	success = true

	return nil
}

// readPageRaw reads a single OGG page, returning both the parsed page and the
// exact raw bytes consumed, so unchanged pages can be copied through verbatim.
func (h *OGGHandler) readPageRaw(r io.Reader) (*oggPage, []byte, error) {
	var raw bytes.Buffer

	page, err := h.readPage(io.TeeReader(r, &raw))
	if err != nil {
		return nil, nil, err
	}

	return page, raw.Bytes(), nil
}

// buildCommentPacket rebuilds a Vorbis ("\x03vorbis") or Opus ("OpusTags")
// comment packet. old is the existing packet and offset the start of its
// vendor-length field. The vendor string and every existing comment whose key
// the new tags don't set are preserved; fields tags sets replace all their
// aliases. Cover art is only replaced when tags carries a new one.
func (h *OGGHandler) buildCommentPacket(old []byte, offset int, opus bool, tags *AudioTags) []byte {
	vendor, existing, rest, ok := parseVorbisCommentList(old[offset:])
	if !ok {
		rest = nil
	}

	if vendor == "" {
		vendor = "go-media-downloader"
	}

	comments, remove := vorbisCommentUpdate(tags)

	if len(tags.CoverData) > 0 {
		remove["METADATA_BLOCK_PICTURE"] = struct{}{}
		remove["COVERART"] = struct{}{}

		comments = append(comments,
			"METADATA_BLOCK_PICTURE="+base64.StdEncoding.EncodeToString(h.buildPictureBlock(tags)))
	}

	for _, c := range existing {
		if _, drop := remove[vorbisCommentKey(c)]; !drop {
			comments = append(comments, c)
		}
	}

	var buf bytes.Buffer

	buf.Write(old[:offset])

	var n [4]byte

	binary.LittleEndian.PutUint32(n[:], uint32(len(vendor))) //nolint:gosec // bounded by packet size
	buf.Write(n[:])
	buf.WriteString(vendor)

	binary.LittleEndian.PutUint32(n[:], uint32(len(comments))) //nolint:gosec // bounded by packet size
	buf.Write(n[:])

	for i := range comments {
		binary.LittleEndian.PutUint32(n[:], uint32(len(comments[i]))) //nolint:gosec // bounded by packet size
		buf.Write(n[:])
		buf.WriteString(comments[i])
	}

	if opus {
		// RFC 7845 §5.2: binary data after the comments whose first byte has
		// the LSB set must be preserved by editors.
		if len(rest) > 0 && rest[0]&1 == 1 {
			buf.Write(rest)
		}
	} else {
		buf.WriteByte(0x01) // Vorbis framing bit
	}

	return buf.Bytes()
}

// buildPictureBlock creates a FLAC-style METADATA_BLOCK_PICTURE.
func (*OGGHandler) buildPictureBlock(tags *AudioTags) []byte {
	var buf bytes.Buffer

	// Picture type (3 = Front Cover)
	binary.Write(&buf, binary.BigEndian, uint32(3))

	// MIME type
	mime := tags.CoverMIME
	if mime == "" {
		mime = "image/jpeg"
	}

	binary.Write(
		&buf,
		binary.BigEndian,
		uint32(len(mime)),
	)
	buf.WriteString(mime)

	// Description
	desc := "Front Cover"
	binary.Write(
		&buf,
		binary.BigEndian,
		uint32(len(desc)),
	)
	buf.WriteString(desc)

	// Width, height, depth, colors (unknown = 0)
	binary.Write(&buf, binary.BigEndian, uint32(0)) // width
	binary.Write(&buf, binary.BigEndian, uint32(0)) // height
	binary.Write(&buf, binary.BigEndian, uint32(0)) // depth
	binary.Write(&buf, binary.BigEndian, uint32(0)) // colors

	// Picture data
	binary.Write(
		&buf,
		binary.BigEndian,
		uint32(len(tags.CoverData)),
	)
	buf.Write(tags.CoverData)

	return buf.Bytes()
}

// buildOggPages paginates packets into Ogg pages starting at sequence number
// seq. Packets are laced into 255-byte segments and pages are filled with up
// to 255 segments each, so packets of any size span as many pages as needed.
// Like libogg, a page on which no packet ends gets granule position -1; header
// pages on which a packet ends get 0. eos sets the end-of-stream flag on the
// last page.
func buildOggPages(packets [][]byte, version uint8, serial, seq uint32, eos bool) [][]byte {
	type oggSeg struct {
		data []byte
		end  bool
	}

	var segs []oggSeg

	for _, p := range packets {
		for off := 0; ; {
			l := min(255, len(p)-off)
			segs = append(segs, oggSeg{data: p[off : off+l], end: l < 255})
			off += l

			if l < 255 {
				break
			}
		}
	}

	var (
		pages     [][]byte
		continued bool
	)

	for start := 0; start < len(segs); start += 255 {
		chunk := segs[start:min(start+255, len(segs))]

		var flags byte
		if continued {
			flags |= 0x01
		}

		if eos && start+len(chunk) == len(segs) {
			flags |= 0x04
		}

		granule := int64(-1)
		lacing := make([]byte, len(chunk))
		bodyLen := 0

		for i := range chunk {
			lacing[i] = byte(len(chunk[i].data)) //nolint:gosec // segment length <= 255
			bodyLen += len(chunk[i].data)

			if chunk[i].end {
				granule = 0
			}
		}

		page := make([]byte, 27+len(lacing)+bodyLen)
		copy(page, "OggS")
		page[4] = version
		page[5] = flags
		binary.LittleEndian.PutUint64(page[6:14], uint64(granule)) //nolint:gosec // -1 is the Ogg "no granule" marker
		binary.LittleEndian.PutUint32(page[14:18], serial)
		binary.LittleEndian.PutUint32(page[18:22], seq)
		page[26] = byte(len(lacing)) //nolint:gosec // <= 255 by construction
		copy(page[27:], lacing)

		pos := 27 + len(lacing)
		for i := range chunk {
			pos += copy(page[pos:], chunk[i].data)
		}

		binary.LittleEndian.PutUint32(page[22:26], oggCRC(page))

		pages = append(pages, page)
		seq++
		continued = !chunk[len(chunk)-1].end
	}

	return pages
}

// copyOggPagesRenumbered copies the remaining Ogg pages from src to dst, adding
// delta to the sequence number of every page of the given logical stream and
// recomputing its CRC. Pages of other streams are copied unchanged. Bytes that
// don't form a complete page (trailing junk, truncation) are copied verbatim.
func copyOggPagesRenumbered(dst io.Writer, src io.Reader, serial, delta uint32) error {
	buf := make([]byte, 0, 27+255+255*255)

	for {
		buf = buf[:27]

		n, err := io.ReadFull(src, buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			if errors.Is(err, io.ErrUnexpectedEOF) {
				_, werr := dst.Write(buf[:n])
				return werr
			}

			return err
		}

		if string(buf[:4]) != "OggS" {
			if _, err := dst.Write(buf); err != nil {
				return err
			}

			_, err := io.Copy(dst, src)

			return err
		}

		nseg := int(buf[26])
		buf = buf[:27+nseg]

		if n, err := io.ReadFull(src, buf[27:]); err != nil {
			return writePartialPage(dst, buf[:27+n], err)
		}

		bodyLen := 0
		for _, l := range buf[27 : 27+nseg] {
			bodyLen += int(l)
		}

		hl := len(buf)
		buf = buf[:hl+bodyLen]

		if n, err := io.ReadFull(src, buf[hl:]); err != nil {
			return writePartialPage(dst, buf[:hl+n], err)
		}

		if binary.LittleEndian.Uint32(buf[14:18]) == serial {
			binary.LittleEndian.PutUint32(buf[18:22], binary.LittleEndian.Uint32(buf[18:22])+delta)
			binary.LittleEndian.PutUint32(buf[22:26], 0)
			binary.LittleEndian.PutUint32(buf[22:26], oggCRC(buf))
		}

		if _, err := dst.Write(buf); err != nil {
			return err
		}
	}
}

// writePartialPage copies the bytes of a truncated final page through
// unchanged; any read error other than truncation is returned.
func writePartialPage(dst io.Writer, partial []byte, err error) error {
	if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return err
	}

	_, werr := dst.Write(partial)

	return werr
}

// oggCRC computes the Ogg page checksum: CRC-32 with polynomial 0x04c11db7,
// MSB-first (non-reflected), initial value 0, no final XOR. The page's CRC
// field must be zero while it is computed.
func oggCRC(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc = (crc << 8) ^ oggCRCTable[byte(crc>>24)^b]
	}

	return crc
}

// oggCRCTable is the CRC32 lookup table for OGG.
var oggCRCTable = [256]uint32{
	0x00000000, 0x04c11db7, 0x09823b6e, 0x0d4326d9,
	0x130476dc, 0x17c56b6b, 0x1a864db2, 0x1e475005,
	0x2608edb8, 0x22c9f00f, 0x2f8ad6d6, 0x2b4bcb61,
	0x350c9b64, 0x31cd86d3, 0x3c8ea00a, 0x384fbdbd,
	0x4c11db70, 0x48d0c6c7, 0x4593e01e, 0x4152fda9,
	0x5f15adac, 0x5bd4b01b, 0x569796c2, 0x52568b75,
	0x6a1936c8, 0x6ed82b7f, 0x639b0da6, 0x675a1011,
	0x791d4014, 0x7ddc5da3, 0x709f7b7a, 0x745e66cd,
	0x9823b6e0, 0x9ce2ab57, 0x91a18d8e, 0x95609039,
	0x8b27c03c, 0x8fe6dd8b, 0x82a5fb52, 0x8664e6e5,
	0xbe2b5b58, 0xbaea46ef, 0xb7a96036, 0xb3687d81,
	0xad2f2d84, 0xa9ee3033, 0xa4ad16ea, 0xa06c0b5d,
	0xd4326d90, 0xd0f37027, 0xddb056fe, 0xd9714b49,
	0xc7361b4c, 0xc3f706fb, 0xceb42022, 0xca753d95,
	0xf23a8028, 0xf6fb9d9f, 0xfbb8bb46, 0xff79a6f1,
	0xe13ef6f4, 0xe5ffeb43, 0xe8bccd9a, 0xec7dd02d,
	0x34867077, 0x30476dc0, 0x3d044b19, 0x39c556ae,
	0x278206ab, 0x23431b1c, 0x2e003dc5, 0x2ac12072,
	0x128e9dcf, 0x164f8078, 0x1b0ca6a1, 0x1fcdbb16,
	0x018aeb13, 0x054bf6a4, 0x0808d07d, 0x0cc9cdca,
	0x7897ab07, 0x7c56b6b0, 0x71159069, 0x75d48dde,
	0x6b93dddb, 0x6f52c06c, 0x6211e6b5, 0x66d0fb02,
	0x5e9f46bf, 0x5a5e5b08, 0x571d7dd1, 0x53dc6066,
	0x4d9b3063, 0x495a2dd4, 0x44190b0d, 0x40d816ba,
	0xaca5c697, 0xa864db20, 0xa527fdf9, 0xa1e6e04e,
	0xbfa1b04b, 0xbb60adfc, 0xb6238b25, 0xb2e29692,
	0x8aad2b2f, 0x8e6c3698, 0x832f1041, 0x87ee0df6,
	0x99a95df3, 0x9d684044, 0x902b669d, 0x94ea7b2a,
	0xe0b41de7, 0xe4750050, 0xe9362689, 0xedf73b3e,
	0xf3b06b3b, 0xf771768c, 0xfa325055, 0xfef34de2,
	0xc6bcf05f, 0xc27dede8, 0xcf3ecb31, 0xcbffd686,
	0xd5b88683, 0xd1799b34, 0xdc3abded, 0xd8fba05a,
	0x690ce0ee, 0x6dcdfd59, 0x608edb80, 0x644fc637,
	0x7a089632, 0x7ec98b85, 0x738aad5c, 0x774bb0eb,
	0x4f040d56, 0x4bc510e1, 0x46863638, 0x42472b8f,
	0x5c007b8a, 0x58c1663d, 0x558240e4, 0x51435d53,
	0x251d3b9e, 0x21dc2629, 0x2c9f00f0, 0x285e1d47,
	0x36194d42, 0x32d850f5, 0x3f9b762c, 0x3b5a6b9b,
	0x0315d626, 0x07d4cb91, 0x0a97ed48, 0x0e56f0ff,
	0x1011a0fa, 0x14d0bd4d, 0x19939b94, 0x1d528623,
	0xf12f560e, 0xf5ee4bb9, 0xf8ad6d60, 0xfc6c70d7,
	0xe22b20d2, 0xe6ea3d65, 0xeba91bbc, 0xef68060b,
	0xd727bbb6, 0xd3e6a601, 0xdea580d8, 0xda649d6f,
	0xc423cd6a, 0xc0e2d0dd, 0xcda1f604, 0xc960ebb3,
	0xbd3e8d7e, 0xb9ff90c9, 0xb4bcb610, 0xb07daba7,
	0xae3afba2, 0xaafbe615, 0xa7b8c0cc, 0xa379dd7b,
	0x9b3660c6, 0x9ff77d71, 0x92b45ba8, 0x9675461f,
	0x8832161a, 0x8cf30bad, 0x81b02d74, 0x857130c3,
	0x5d8a9099, 0x594b8d2e, 0x5408abf7, 0x50c9b640,
	0x4e8ee645, 0x4a4ffbf2, 0x470cdd2b, 0x43cdc09c,
	0x7b827d21, 0x7f436096, 0x7200464f, 0x76c15bf8,
	0x68860bfd, 0x6c47164a, 0x61043093, 0x65c52d24,
	0x119b4be9, 0x155a565e, 0x18197087, 0x1cd86d30,
	0x029f3d35, 0x065e2082, 0x0b1d065b, 0x0fdc1bec,
	0x3793a651, 0x3352bbe6, 0x3e119d3f, 0x3ad08088,
	0x2497d08d, 0x2056cd3a, 0x2d15ebe3, 0x29d4f654,
	0xc5a92679, 0xc1683bce, 0xcc2b1d17, 0xc8ea00a0,
	0xd6ad50a5, 0xd26c4d12, 0xdf2f6bcb, 0xdbee767c,
	0xe3a1cbc1, 0xe760d676, 0xea23f0af, 0xeee2ed18,
	0xf0a5bd1d, 0xf464a0aa, 0xf9278673, 0xfde69bc4,
	0x89b8fd09, 0x8d79e0be, 0x803ac667, 0x84fbdbd0,
	0x9abc8bd5, 0x9e7d9662, 0x933eb0bb, 0x97ffad0c,
	0xafb010b1, 0xab710d06, 0xa6322bdf, 0xa2f33668,
	0xbcb4666d, 0xb8757bda, 0xb5365d03, 0xb1f740b4,
}

// GetDuration attempts to calculate the duration of an OGG file.
func (h *OGGHandler) GetDuration(filepath string) (time.Duration, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	// Sample rate comes from the first page's identification header. Opus always
	// timestamps granule positions at 48 kHz regardless of the input rate.
	var sampleRate int

	if page, perr := h.readPage(file); perr == nil {
		switch {
		case len(page.Data) >= 19 && string(page.Data[0:8]) == "OpusHead":
			sampleRate = 48000
		case len(page.Data) > 16 && page.Data[0] == 0x01 && string(page.Data[1:7]) == "vorbis":
			sampleRate = int(binary.LittleEndian.Uint32(page.Data[12:16]))
		}
	}

	if sampleRate == 0 {
		return 0, errors.New("could not determine sample rate")
	}

	// The final granule position is the total sample count. Scan the tail of the
	// file for Ogg page captures and take the largest granule (seeking to a fixed
	// offset would land mid-page and miss the capture pattern).
	readSize := int64(65536)
	if stat.Size() < readSize {
		readSize = stat.Size()
	}

	if _, err := file.Seek(stat.Size()-readSize, io.SeekStart); err != nil {
		return 0, err
	}

	tail := make([]byte, readSize)

	nn, err := io.ReadFull(file, tail)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return 0, err
	}

	tail = tail[:nn]

	lastGranule := int64(-1)

	for i := 0; i+14 <= len(tail); i++ {
		// Match "OggS" with stream-structure version 0 to avoid false positives.
		if tail[i] != 'O' || tail[i+1] != 'g' || tail[i+2] != 'g' || tail[i+3] != 'S' ||
			tail[i+4] != 0 {
			continue
		}

		if gp := int64(binary.LittleEndian.Uint64(tail[i+6 : i+14])); gp > lastGranule {
			lastGranule = gp
		}
	}

	if lastGranule > 0 {
		seconds := float64(lastGranule) / float64(sampleRate)
		return time.Duration(seconds * float64(time.Second)), nil
	}

	return 0, errors.New("could not determine duration")
}
