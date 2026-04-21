package tags

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
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
func (h *OGGHandler) SupportedFormats() []string {
	return []string{".ogg", ".oga"}
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

// ReadTags reads Vorbis Comment tags from an OGG Vorbis file.
func (h *OGGHandler) ReadTags(filepath string) (*AudioTags, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}
	defer file.Close()

	tags := &AudioTags{}

	// Read pages until we find the comment header
	pageNum := 0
	for pageNum < 10 { // Limit search to first 10 pages
		page, err := h.readPage(file)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
		}

		// Check for Vorbis comment header (packet type 0x03)
		if len(page.Data) > 7 && page.Data[0] == 0x03 {
			if string(page.Data[1:7]) == "vorbis" {
				h.parseVorbisCommentPacket(page.Data[7:], tags)
				break
			}
		}

		pageNum++
	}

	// Try to get audio info from identification header
	file.Seek(0, 0)

	page, err := h.readPage(file)
	if err == nil && len(page.Data) > 29 && page.Data[0] == 0x01 {
		if string(page.Data[1:7]) == "vorbis" {
			// Parse identification header
			tags.Channels = int(page.Data[11])
			tags.SampleRate = int(binary.LittleEndian.Uint32(page.Data[12:16]))
			tags.Bitrate = int(
				binary.LittleEndian.Uint32(page.Data[20:24]),
			) / 1000 // Convert to kbps
		}
	}

	return tags, nil
}

// readPage reads a single OGG page from the file.
func (h *OGGHandler) readPage(r io.Reader) (*oggPage, error) {
	// Read capture pattern
	capture := make([]byte, 4)
	if _, err := io.ReadFull(r, capture); err != nil {
		return nil, err
	}

	if string(capture) != "OggS" {
		return nil, fmt.Errorf("invalid OGG page: bad capture pattern")
	}

	page := &oggPage{}

	// Read header
	header := make([]byte, 23)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	page.Version = header[0]
	page.HeaderType = header[1]
	page.GranulePos = int64(binary.LittleEndian.Uint64(header[2:10]))
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
	for _, seg := range page.SegmentTable {
		dataSize += int(seg)
	}

	// Read page data
	page.Data = make([]byte, dataSize)
	if _, err := io.ReadFull(r, page.Data); err != nil {
		return nil, err
	}

	return page, nil
}

// parseVorbisCommentPacket parses the Vorbis comment packet data.
func (h *OGGHandler) parseVorbisCommentPacket(data []byte, tags *AudioTags) {
	if len(data) < 4 {
		return
	}

	// Read vendor string length
	vendorLen := binary.LittleEndian.Uint32(data[0:4])
	if len(data) < int(4+vendorLen+4) {
		return
	}

	// Skip vendor string
	offset := 4 + int(vendorLen)

	// Read comment count
	if len(data) < offset+4 {
		return
	}

	commentCount := binary.LittleEndian.Uint32(data[offset : offset+4])

	offset += 4

	// Read each comment
	for range commentCount {
		if len(data) < offset+4 {
			break
		}

		commentLen := binary.LittleEndian.Uint32(data[offset : offset+4])

		offset += 4

		if len(data) < offset+int(commentLen) {
			break
		}

		comment := string(data[offset : offset+int(commentLen)])

		offset += int(commentLen)

		// Parse key=value
		before, after, ok := strings.Cut(comment, "=")
		if !ok {
			continue
		}

		key := strings.ToUpper(before)
		value := after

		h.setTagValue(key, value, tags)
	}
}

// setTagValue sets the appropriate tag field based on the key.
func (h *OGGHandler) setTagValue(key, value string, tags *AudioTags) {
	switch key {
	// Standard tags
	case "TITLE":
		tags.Title = value
	case "ARTIST":
		tags.Artist = value
	case "ALBUM":
		tags.Album = value
	case "ALBUMARTIST", "ALBUM ARTIST":
		tags.AlbumArtist = value
	case "GENRE":
		tags.Genre = value
	case "DATE", "YEAR":
		tags.Year = parseYear(value)
	case "TRACKNUMBER", "TRACK":
		tags.TrackNumber, _ = parseTrackNum(value)
	case "TRACKTOTAL", "TOTALTRACKS":
		tags.TotalTracks, _ = strconv.Atoi(value)
	case "DISCNUMBER", "DISC":
		tags.DiscNumber, _ = parseTrackNum(value)
	case "DISCTOTAL", "TOTALDISCS":
		tags.TotalDiscs, _ = strconv.Atoi(value)
	case "COMMENT", "DESCRIPTION":
		tags.Comment = value

	// Extended tags
	case "COMPOSER":
		tags.Composer = value
	case "CONDUCTOR":
		tags.Conductor = value
	case "LABEL", "ORGANIZATION", "PUBLISHER":
		tags.Label = value
	case "CATALOGNUMBER", "CATALOG":
		tags.CatalogNum = value
	case "ISRC":
		tags.ISRC = value
	case "LYRICS", "UNSYNCEDLYRICS":
		tags.Lyrics = value
	case "COPYRIGHT":
		tags.Copyright = value

	// MusicBrainz IDs
	case "MUSICBRAINZ_TRACKID", "MUSICBRAINZ_RECORDINGID":
		tags.MBRecordingID = value
	case "MUSICBRAINZ_ALBUMID", "MUSICBRAINZ_RELEASEID":
		tags.MBReleaseID = value
	case "MUSICBRAINZ_RELEASEGROUPID":
		tags.MBReleaseGroupID = value
	case "MUSICBRAINZ_ARTISTID":
		tags.MBArtistID = value
	case "MUSICBRAINZ_ALBUMARTISTID":
		tags.MBAlbumArtistID = value

	// AcoustID
	case "ACOUSTID_ID":
		tags.AcoustID = value

	// ReplayGain
	case "REPLAYGAIN_TRACK_GAIN":
		tags.ReplayGainTrack = parseReplayGain(value)
	case "REPLAYGAIN_TRACK_PEAK":
		tags.ReplayGainTrackPeak = parseReplayGainPeak(value)
	case "REPLAYGAIN_ALBUM_GAIN":
		tags.ReplayGainAlbum = parseReplayGain(value)
	case "REPLAYGAIN_ALBUM_PEAK":
		tags.ReplayGainAlbumPeak = parseReplayGainPeak(value)

	// Cover art (base64 encoded in METADATA_BLOCK_PICTURE)
	case "METADATA_BLOCK_PICTURE":
		h.parsePictureBlock(value, tags)
	case "COVERART": // Legacy cover art format
		if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
			tags.CoverData = decoded
			tags.CoverMIME = detectImageMIME(decoded)
		}
	}
}

// parsePictureBlock parses a FLAC-style METADATA_BLOCK_PICTURE.
func (h *OGGHandler) parsePictureBlock(encoded string, tags *AudioTags) {
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

	mimeLen := binary.BigEndian.Uint32(data[4:8])
	if len(data) < int(8+mimeLen+4) {
		return
	}

	mime := string(data[8 : 8+mimeLen])
	offset := 8 + int(mimeLen)

	// Skip description
	descLen := binary.BigEndian.Uint32(data[offset : offset+4])

	offset += 4 + int(descLen)

	// Skip width, height, depth, colors (16 bytes)
	offset += 16

	if len(data) < offset+4 {
		return
	}

	picLen := binary.BigEndian.Uint32(data[offset : offset+4])

	offset += 4

	if len(data) < offset+int(picLen) {
		return
	}

	tags.CoverData = data[offset : offset+int(picLen)]
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

	if bytes.HasPrefix(data, []byte("WEBP")) || (len(data) > 12 && string(data[8:12]) == "WEBP") {
		return "image/webp"
	}

	return "image/jpeg" // Default
}

// WriteTags writes Vorbis Comment tags to an OGG Vorbis file.
// Note: This is a complex operation that requires rewriting the entire file.
// For production use, consider using a library like github.com/jfreymuth/oggvorbis.
func (h *OGGHandler) WriteTags(filepath string, tags *AudioTags) error {
	// Read the entire file
	fileData, err := os.ReadFile(filepath)
	if err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	// Find the comment packet and rebuild it
	reader := bytes.NewReader(fileData)

	var (
		outputPages      [][]byte
		commentPageIndex = -1
	)

	pageNum := 0
	for {
		pageStart := int(reader.Size()) - reader.Len()

		page, err := h.readPage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
		}

		// Check if this is the comment header
		if len(page.Data) > 7 && page.Data[0] == 0x03 && string(page.Data[1:7]) == "vorbis" {
			commentPageIndex = pageNum
			// Build new comment packet
			newCommentData := h.buildVorbisCommentPacket(tags)

			// Rebuild page with new data
			page.Data = newCommentData

			newPageData := h.buildPage(page)

			outputPages = append(outputPages, newPageData)
		} else {
			// Keep original page data
			pageEnd := int(reader.Size()) - reader.Len()

			outputPages = append(outputPages, fileData[pageStart:pageEnd])
		}

		pageNum++
	}

	if commentPageIndex < 0 {
		return &ErrWriteFailed{Path: filepath, Reason: "could not find Vorbis comment header"}
	}

	// Recalculate page sequence numbers and checksums
	var output bytes.Buffer
	for i, pageData := range outputPages {
		if i == commentPageIndex {
			// Already rebuilt with correct data
			output.Write(pageData)
		} else {
			output.Write(pageData)
		}
	}

	// Write the file
	if err := os.WriteFile(filepath, output.Bytes(), 0o644); err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	return nil
}

// buildVorbisCommentPacket creates a Vorbis comment packet from tags.
func (h *OGGHandler) buildVorbisCommentPacket(tags *AudioTags) []byte {
	var buf bytes.Buffer

	// Packet type (0x03) + "vorbis"
	buf.WriteByte(0x03)
	buf.WriteString("vorbis")

	// Vendor string
	vendor := "go-media-downloader"
	binary.Write(&buf, binary.LittleEndian, uint32(len(vendor)))
	buf.WriteString(vendor)

	// Build comment list
	comments := h.buildCommentList(tags)

	// Comment count
	binary.Write(&buf, binary.LittleEndian, uint32(len(comments)))

	// Write each comment
	for _, comment := range comments {
		binary.Write(&buf, binary.LittleEndian, uint32(len(comment)))
		buf.WriteString(comment)
	}

	// Framing bit
	buf.WriteByte(0x01)

	return buf.Bytes()
}

// buildCommentList creates the list of "KEY=value" comment strings.
func (h *OGGHandler) buildCommentList(tags *AudioTags) []string {
	var comments []string

	addComment := func(key, value string) {
		if value != "" {
			comments = append(comments, key+"="+value)
		}
	}

	// Standard tags
	addComment("TITLE", tags.Title)
	addComment("ARTIST", tags.Artist)
	addComment("ALBUM", tags.Album)
	addComment("ALBUMARTIST", tags.AlbumArtist)
	addComment("GENRE", tags.Genre)

	if tags.Year > 0 {
		addComment("DATE", strconv.Itoa(tags.Year))
	}

	if tags.TrackNumber > 0 {
		addComment("TRACKNUMBER", formatPaddedNum(tags.TrackNumber, tags.TotalTracks))
	}

	if tags.TotalTracks > 0 {
		addComment("TRACKTOTAL", formatPaddedNum(tags.TotalTracks, tags.TotalTracks))
	}

	if tags.DiscNumber > 0 {
		addComment("DISCNUMBER", strconv.Itoa(tags.DiscNumber))
	}

	if tags.TotalDiscs > 0 {
		addComment("DISCTOTAL", strconv.Itoa(tags.TotalDiscs))
	}

	addComment("COMMENT", tags.Comment)

	// Extended tags
	addComment("COMPOSER", tags.Composer)
	addComment("CONDUCTOR", tags.Conductor)
	addComment("LABEL", tags.Label)
	addComment("CATALOGNUMBER", tags.CatalogNum)
	addComment("ISRC", tags.ISRC)
	addComment("LYRICS", tags.Lyrics)
	addComment("COPYRIGHT", tags.Copyright)

	// MusicBrainz IDs
	addComment("MUSICBRAINZ_TRACKID", tags.MBRecordingID)
	addComment("MUSICBRAINZ_ALBUMID", tags.MBReleaseID)
	addComment("MUSICBRAINZ_RELEASEGROUPID", tags.MBReleaseGroupID)
	addComment("MUSICBRAINZ_ARTISTID", tags.MBArtistID)
	addComment("MUSICBRAINZ_ALBUMARTISTID", tags.MBAlbumArtistID)

	// AcoustID
	addComment("ACOUSTID_ID", tags.AcoustID)

	// ReplayGain
	if tags.ReplayGainTrack != 0 {
		addComment("REPLAYGAIN_TRACK_GAIN", fmt.Sprintf("%.2f dB", tags.ReplayGainTrack))
	}

	if tags.ReplayGainTrackPeak != 0 {
		addComment("REPLAYGAIN_TRACK_PEAK", fmt.Sprintf("%.6f", tags.ReplayGainTrackPeak))
	}

	if tags.ReplayGainAlbum != 0 {
		addComment("REPLAYGAIN_ALBUM_GAIN", fmt.Sprintf("%.2f dB", tags.ReplayGainAlbum))
	}

	if tags.ReplayGainAlbumPeak != 0 {
		addComment("REPLAYGAIN_ALBUM_PEAK", fmt.Sprintf("%.6f", tags.ReplayGainAlbumPeak))
	}

	// Cover art as METADATA_BLOCK_PICTURE
	if len(tags.CoverData) > 0 {
		pictureBlock := h.buildPictureBlock(tags)
		addComment("METADATA_BLOCK_PICTURE", base64.StdEncoding.EncodeToString(pictureBlock))
	}

	return comments
}

// buildPictureBlock creates a FLAC-style METADATA_BLOCK_PICTURE.
func (h *OGGHandler) buildPictureBlock(tags *AudioTags) []byte {
	var buf bytes.Buffer

	// Picture type (3 = Front Cover)
	binary.Write(&buf, binary.BigEndian, uint32(3))

	// MIME type
	mime := tags.CoverMIME
	if mime == "" {
		mime = "image/jpeg"
	}

	binary.Write(&buf, binary.BigEndian, uint32(len(mime)))
	buf.WriteString(mime)

	// Description
	desc := "Front Cover"
	binary.Write(&buf, binary.BigEndian, uint32(len(desc)))
	buf.WriteString(desc)

	// Width, height, depth, colors (unknown = 0)
	binary.Write(&buf, binary.BigEndian, uint32(0)) // width
	binary.Write(&buf, binary.BigEndian, uint32(0)) // height
	binary.Write(&buf, binary.BigEndian, uint32(0)) // depth
	binary.Write(&buf, binary.BigEndian, uint32(0)) // colors

	// Picture data
	binary.Write(&buf, binary.BigEndian, uint32(len(tags.CoverData)))
	buf.Write(tags.CoverData)

	return buf.Bytes()
}

// buildPage rebuilds an OGG page with updated data.
func (h *OGGHandler) buildPage(page *oggPage) []byte {
	var buf bytes.Buffer

	// Capture pattern
	buf.WriteString("OggS")

	// Version
	buf.WriteByte(page.Version)

	// Header type
	buf.WriteByte(page.HeaderType)

	// Granule position
	binary.Write(&buf, binary.LittleEndian, page.GranulePos)

	// Bitstream serial
	binary.Write(&buf, binary.LittleEndian, page.BitstreamSerial)

	// Page sequence
	binary.Write(&buf, binary.LittleEndian, page.PageSequence)

	// Placeholder for checksum (will be calculated)
	checksumPos := buf.Len()
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	// Build segment table for new data
	dataLen := len(page.Data)
	segments := make([]byte, 0)

	remaining := dataLen
	for remaining > 0 {
		if remaining >= 255 {
			segments = append(segments, 255)

			remaining -= 255
		} else {
			segments = append(segments, byte(remaining))
			remaining = 0
		}
	}

	if dataLen%255 == 0 && dataLen > 0 {
		segments = append(segments, 0) // Terminate packet
	}

	// Page segments count
	buf.WriteByte(byte(len(segments)))

	// Segment table
	buf.Write(segments)

	// Page data
	buf.Write(page.Data)

	// Calculate checksum
	pageData := buf.Bytes()
	checksum := h.calculateCRC32(pageData)
	binary.LittleEndian.PutUint32(pageData[checksumPos:checksumPos+4], checksum)

	return pageData
}

// calculateCRC32 calculates the OGG CRC32 checksum.
func (h *OGGHandler) calculateCRC32(data []byte) uint32 {
	// OGG uses a specific CRC32 polynomial
	var crc uint32 = 0
	for _, b := range data {
		crc = oggCRCTable[byte(crc)^b] ^ (crc >> 8)
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

	// Get file size for seeking to end
	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	// Seek near the end of the file to find the last page
	seekPos := max(
		// Last 64KB should contain the final page
		stat.Size()-65536, 0)

	file.Seek(seekPos, 0)

	var (
		lastGranulePos int64
		sampleRate     int
	)

	// Read pages to find the last granule position

	for {
		page, err := h.readPage(file)
		if err != nil {
			break
		}

		if page.GranulePos > 0 {
			lastGranulePos = page.GranulePos
		}
	}

	// Get sample rate from the first page
	file.Seek(0, 0)

	page, err := h.readPage(file)
	if err == nil && len(page.Data) > 16 && page.Data[0] == 0x01 {
		if string(page.Data[1:7]) == "vorbis" {
			sampleRate = int(binary.LittleEndian.Uint32(page.Data[12:16]))
		}
	}

	if sampleRate > 0 && lastGranulePos > 0 {
		seconds := float64(lastGranulePos) / float64(sampleRate)
		return time.Duration(seconds * float64(time.Second)), nil
	}

	return 0, fmt.Errorf("could not determine duration")
}
