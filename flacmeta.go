// Package flacmeta provides an API to extract and print metadata from FLAC
// audio files.
package flacmeta

// TODO:
// 	FLACParseCuesheetBlock()
//	FLACParseCuesheetTrack()

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// METADATA_BLOCK_HEADER_TYPES enumerates the types of metadata blocks in a
// FLAC file.
var METADATA_BLOCK_HEADER_TYPES = map[uint32]string{
	0:   "STREAMINFO",
	1:   "PADDING",
	2:   "APPLICATION",
	3:   "SEEKTABLE",
	4:   "VORBIS_COMMENT",
	5:   "CUESHEET",
	6:   "PICTURE",
	127: "INVALID",
}

// PictureTypeMap enumerates the types of pictures in a FLACPictureBlock.
var PictureTypeMap = map[uint32]string{
	0:  "Other",
	1:  "File Icon",
	2:  "Other File Icon",
	3:  "Cover (front)",
	4:  "Cover (back)",
	5:  "Leaflet Page",
	6:  "Media",
	7:  "Lead Artist/Lead Performer/Soloist",
	8:  "Artist/Performer",
	9:  "Conductor",
	10: "Band/Orchestra",
	11: "Composer",
	12: "Lyricist/Text Writer",
	13: "Recording Location",
	14: "During Recording",
	15: "During Performance",
	16: "Movie/Video Screen Capture",
	17: "A Bright Coloured Fish",
	18: "Illustration",
	19: "Band/Artist Logotype",
	20: "Publisher/Studio Logotype",
}

// Begin convenience functions.

// LookupHeaderType looks up the type of a metadata block header based on
// numeric id.
func LookupHeaderType(k uint32) (bool, string) {
	blkType := METADATA_BLOCK_HEADER_TYPES[k]

	switch blkType {
	case "":
		return true, "FATAL: Invalid block header type."
	}
	return true, blkType
}

// LookupHeaderType looks up the type of a picture based on numeric id.
func LookupPictureType(k uint32) string {
	t := PictureTypeMap[k]
	switch t {
	case "":
		return "UNKNOWN"
	}
	return t
}

// Here starts the base metadata block types.

// FLACApplicationBlock contains the ID and binary data of an embedded
// executable.
// Only one ApplicationBlock is allowed per file.
type FLACApplicationBlock struct {
	Id   uint32
	Data []byte
}

// FLACMetadataBlockHeader is the common element for every metadata block in a
// FLAC file. It describes what kind of block the metadata is, how long (in bytes)
// it is and if it is the last metadata block before the start of the audio stream.
type FLACMetadataBlockHeader struct {
	Type       uint32
	Length     uint32
	Last       bool
	SeekPoints uint16
}

// FLACPictureBlock contains information and binary data about pictures that
// are embedded in the FLAC file. Muitiple PictureBlocks are allow per file.
type FLACPictureBlock struct {
	PictureType        string
	MimeType           string
	PictureDescription string
	Width              uint32
	Height             uint32
	ColorDepth         uint32
	NumColors          uint32
	Length             uint32
	PictureBlob        string
}

// FLACStreaminfoBlock contains information on the audio stream.  Only one
// StreaminfoBlock is allowed per file.
type FLACStreaminfoBlock struct {
	MinBlockSize  uint16
	MaxBlockSize  uint16
	MinFrameSize  uint32
	MaxFrameSize  uint32
	SampleRate    uint32
	Channels      uint8
	BitsPerSample uint8
	TotalSamples  uint64
	MD5Signature  string
}

// FLACVorbisCommentBlock contains general information about the song/audio.
// Common fields are Artist, Song Title and Album. Only one VorbisCommentBlock
// is allowed per file.
type FLACVorbisCommentBlock struct {
	Vendor        string
	TotalComments uint32
	Comments      []string
}

// Here starts the full metadata blocks: a struct containing a
// FLACMetadataBlockHeader and the corresponding metadata block. Structs with
// an IsPopulated field imply that only one of this type of block is allowed in
// a FLAC file.

// FLACApplication is a full Application block (header + data).
type FLACApplication struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACApplicationBlock
	IsPopulated bool
}

// FLACCuesheet describes a "Cue sheet".
type FLACCuesheet struct {
	Header      *FLACMetadataBlockHeader
	Data        []byte
	IsPopulated bool
}

// FLACPadding is a full Padding block (header + data).
type FLACPadding struct {
	Header      *FLACMetadataBlockHeader
	Data        []byte
	IsPopulated bool
}

// FLACPicture is a full Picture block (header + data).
type FLACPicture struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACPictureBlock
	IsPopulated bool
}

// FLACSeekpointBlock contains locations within the FLAC file that allows
// an application to quickly jump to pre-defined locations in the audio stream.
type FLACSeekpointBlock struct {
	SampleNumber uint64
	Offset       uint64
	FrameSamples uint16
}

// FLACSeektable is a full Seek Table block (header + data).
type FLACSeektable struct {
	Header      *FLACMetadataBlockHeader
	Data        []FLACSeekpointBlock
	IsPopulated bool
}

// FLACStreaminfo is a full streaminfo block (header + data).
type FLACStreaminfo struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACStreaminfoBlock
	IsPopulated bool
}

// FLACVorbisComment is a full Vorbis Comment block (header + data).
type FLACVorbisComment struct {
	Header      *FLACMetadataBlockHeader
	Data        *FLACVorbisCommentBlock
	IsPopulated bool
}

// FLACMetadata represents all metadata present in a FLAC file.
type FLACMetadata struct {
	FLACStreaminfo
	FLACApplication
	FLACVorbisComment
	FLACPictures []FLACPicture
	FLACPadding
	FLACSeektable // not implemented yet
	FLACCuesheet  // not implemented yet
	TotalBlocks   uint8
}

// Begin FLACParseX functions.

// FLACParseApplicationBlock parses the bits from an application block.
func (ab *FLACApplicationBlock) FLACParseApplicationBlock(block []byte) (bool, string) {
	buf := bytes.NewBuffer(block)

	ab.Id = binary.BigEndian.Uint32(buf.Next(4))
	if buf.Len()%8 != 0 {
		return true, "Malformed METADATA_BLOCK_APPLICATION: the data field length is not a mulitple of 8"
	}
	ab.Data = buf.Bytes()
	return false, ""
}

// FLACParseMetadataBlockHeader parses the bits from a FLAC metadata block header.
func (mbh *FLACMetadataBlockHeader) FLACParseMetadataBlockHeader(block []byte) (bool, string) {
	var LASTBLOCK uint32 = 0x80000000
	var BLOCKTYPE uint32 = 0x7F000000
	var BLOCKLEN uint32 = 0x00FFFFFF

	bits := binary.BigEndian.Uint32(block)

	mbh.Type = (BLOCKTYPE & bits) >> 24
	mbh.Length = BLOCKLEN & bits
	mbh.Last = false
	if (LASTBLOCK&bits)>>31 == 1 {
		mbh.Last = true
	}
	if mbh.Type == 3 {
		if mbh.Length % 18 != 0 {
			return true, "SEEKTABLE block length is not a multiple of 18."
		}
		mbh.SeekPoints = uint16(mbh.Length / 18)
	}

	return false, ""
}

// FLACParsePictureBlock parses the bits from a picture block.
func (pb *FLACPictureBlock) FLACParsePictureBlock(block []byte) {
	// bits   Field
	// -----+------
	// <32>   The picture type according to the ID3v2 APIC frame
	// <32>   The length of the MIME type string in bytes.
	// <n*8>  The MIME type string, in printable ASCII characters 0x20-0x7e. The MIME type may also be --> to signify that the data part is a URL of the picture instead of the picture data itself.
	// <32>   The length of the description string in bytes.
	// <n*8>  The description of the picture, in UTF-8.
	// <32>   The width of the picture in pixels.
	// <32>   The height of the picture in pixels.
	// <32>   The color depth of the picture in bits-per-pixel.
	// <32>   For indexed-color pictures (e.g. GIF), the number of colors used, or 0 for non-indexed pictures.
	// <32>   The length of the picture data in bytes.
	// <n*8>  The binary picture data.

	buf := bytes.NewBuffer(block)

	pb.PictureType = LookupPictureType(binary.BigEndian.Uint32(buf.Next(4)))

	mimeLen := int(binary.BigEndian.Uint32(buf.Next(4)))
	pb.MimeType = string(buf.Next(mimeLen))

	descLen := int(binary.BigEndian.Uint32(buf.Next(4)))
	if descLen > 0 {
		pb.PictureDescription = string(binary.BigEndian.Uint32(buf.Next(descLen)))
	} else {
		pb.PictureDescription = ""
	}
	pb.Width = binary.BigEndian.Uint32(buf.Next(4))
	pb.Height = binary.BigEndian.Uint32(buf.Next(4))
	pb.ColorDepth = binary.BigEndian.Uint32(buf.Next(4))
	pb.NumColors = binary.BigEndian.Uint32(buf.Next(4))
	pb.Length = binary.BigEndian.Uint32(buf.Next(4))
	pb.PictureBlob = hex.Dump(buf.Next(int(pb.Length)))
}

// FLACParseSeekpointBlock parses the bits from a FLAC seekpoint block.
func FLACParseSeekpointBlock(block []byte) (spb []FLACSeekpointBlock) {
	buf := bytes.NewBuffer(block)
	var sample uint64
	var offset uint64
	var frameSamp uint16
	
	for ; buf.Len() > 0; {
		sample = binary.BigEndian.Uint64(buf.Next(8))
		offset = binary.BigEndian.Uint64(buf.Next(8))
		frameSamp = binary.BigEndian.Uint16(buf.Next(2))

		spb = append(spb, FLACSeekpointBlock{sample, offset, frameSamp})
	}
	return spb
}

// FLACParseStreaminfoBlock parses the bits from a FLAC streaminfo block.
func (sib *FLACStreaminfoBlock) FLACParseStreaminfoBlock(block []byte) {
	// From: http://flac.sourceforge.net/format.html
	// The FLAC STREAMINFO block is structured thus:
	// <16>  - Minimum block size (in samples) used in the stream.
	// <16>  - Maximum block size (in samples) used in the stream.
	// <24>  - Minimum frame size (in bytes) used in the stream. 0 == Implied Unknown
	// <24>  - Maximum frame size (in bytes) used in the stream. 0 == Implied Unknown
	// <20>  - Sample rate (in Hz). Must be > 0 && < 655350
	// <3>   - Number of channels - 1. Why -1?
	// <5>   - Bits per sample - 1. Why -1?
	// <36>  - Total number of samples in the stream. 0 == Implied Unknown
	// <128> - MD5 signature of the unencoded audio data.
	//
	// In order to keep everything on powers-of-2 boundaries, reads from the
	// block are grouped thus:
	//
	// 	MinBlockSize = 16 bits
	// 	MaxBlockSize + minFrameSize + maxFrameSize = 64 bits
	// 	SampleRate + channels + bitsPerSample + TotalSamples = 64 bits
	// 	md5Signature = 128 bits

	buf := bytes.NewBuffer(block)

	var (
		bits            uint64
		minFSMask       uint64 = 0xFFFFFFFFFFFFFFFF
		maxFSMask       uint64 = 0xFFFFFF
		sampRateMask    uint64 = 0xFFFFF00000000000
		bitsPerSampMask uint64 = 0x1F000000000
		chMask          uint64 = 0xE0000000000
		totSampMask     uint64 = 0xFFFFFFFFF
	)

	sib.MinBlockSize = binary.BigEndian.Uint16(buf.Next(2))

	bits = binary.BigEndian.Uint64(buf.Next(8))
	sib.MaxBlockSize = uint16((minFSMask & bits) >> 48)
	sib.MinFrameSize = uint32((minFSMask & bits) >> 24)
	sib.MaxFrameSize = uint32(maxFSMask & bits)

	bits = binary.BigEndian.Uint64(buf.Next(8))
	sib.SampleRate = uint32((sampRateMask & bits) >> 44)
	sib.Channels = uint8((chMask&bits)>>41) + 1
	sib.BitsPerSample = uint8((bitsPerSampMask&bits)>>36) + 1
	sib.TotalSamples = bits & totSampMask

	sib.MD5Signature = fmt.Sprintf("%x", buf.Next(16))
}

// FLACParseVorbisCommentBlock parses the bits in a Vorbis comment block.
func (vcb *FLACVorbisCommentBlock) FLACParseVorbisCommentBlock(block []byte) {
	/* http://www.xiph.org/vorbis/doc/v-comment.html
	The comment header is decoded as follows:

		1) [vendor_length] = read an unsigned integer of 32 bits
		2) [vendor_string] = read a UTF-8 vector as [vendor_length] octets
		3) [user_comment_list_length] = read an unsigned integer of 32 bits
		4) iterate [user_comment_list_length] times {
			5) [length] = read an unsigned integer of 32 bits
			6) this iteration's user comment = read a UTF-8 vector as [length] octets
		}
		7) done.
	*/

	buf := bytes.NewBuffer(block)

	vendorLen := int(binary.LittleEndian.Uint32(buf.Next(4)))
	vcb.Vendor = string(buf.Next(vendorLen))

	vcb.TotalComments = binary.LittleEndian.Uint32(buf.Next(4))

	for tc := vcb.TotalComments; tc > 0; tc-- {
		commentLen := int(binary.LittleEndian.Uint32(buf.Next(4)))
		comment := string(buf.Next(commentLen))
		vcb.Comments = append(vcb.Comments, comment)
	}
}

func (data *FLACMetadata) PrintFLACApplicationData() {
	data.FLACApplication.Header.PrintFLACMetadataBlockHeader()
	fmt.Println("  app. id:", string(data.FLACApplication.Data.Id))
}

func (header *FLACMetadataBlockHeader) PrintFLACMetadataBlockHeader() {
	_, hdrType := LookupHeaderType(header.Type)
	fmt.Printf("  type: %d (%s)\n", header.Type, hdrType)
	fmt.Println("  ls last:", header.Last)
	fmt.Println("  length:", header.Length)
}

func (data *FLACMetadata) PrintFLACPaddingData() {
	data.FLACPadding.Header.PrintFLACMetadataBlockHeader()
}

func (data *FLACMetadata) PrintFLACPictureData() {
	for _, entry := range data.FLACPictures {
		entry.Header.PrintFLACMetadataBlockHeader()
		fmt.Println("  type:", entry.Data.PictureType)
		fmt.Println("  MIME type:", entry.Data.MimeType)
		fmt.Println("  description:", entry.Data.PictureDescription)
		fmt.Println("  width:", entry.Data.Width)
		fmt.Println("  height:", entry.Data.Height)
		fmt.Println("  depth:", entry.Data.ColorDepth)
		fmt.Println("  colors:", entry.Data.NumColors)
		fmt.Println("  data length:", entry.Data.Length)
		for _, l := range strings.Split(entry.Data.PictureBlob, "\n") {
			fmt.Println("   ", l)
		}
	}
}

func (data *FLACMetadata) PrintFLACStreaminfoBlockData() {
	data.FLACStreaminfo.Header.PrintFLACMetadataBlockHeader()
	fmt.Println("  minimum blocksize:", data.FLACStreaminfo.Data.MinBlockSize, "samples")
	fmt.Println("  maximum blocksize:", data.FLACStreaminfo.Data.MaxBlockSize, "samples")
	fmt.Println("  minimum framesize:", data.FLACStreaminfo.Data.MinFrameSize, "bytes")
	fmt.Println("  maximum framesize:", data.FLACStreaminfo.Data.MaxFrameSize, "bytes")
	fmt.Println("  sample_rate:", data.FLACStreaminfo.Data.SampleRate)
	fmt.Println("  channels:", data.FLACStreaminfo.Data.Channels)
	fmt.Println("  bits-per-sample:", data.FLACStreaminfo.Data.BitsPerSample)
	fmt.Println("  total samples:", data.FLACStreaminfo.Data.TotalSamples)
	fmt.Println("  MD5 signature:", data.FLACStreaminfo.Data.MD5Signature)
}

func (data *FLACMetadata) PrintFLACVorbisCommentData() {
	data.FLACVorbisComment.Header.PrintFLACMetadataBlockHeader()
	fmt.Println("  vendor string:", data.FLACVorbisComment.Data.Vendor)
	fmt.Println("  comments:", data.FLACVorbisComment.Data.TotalComments)
	for i, v := range data.FLACVorbisComment.Data.Comments {
		fmt.Printf("    comment[%d]: %s\n", i, v)
	}
}

// ReadFLACMetatada reads the metadata from a flac file and populates a
// FLACMetadata struct with the data.
func (flacmetadata *FLACMetadata) ReadFLACMetadata(f *os.File) (bool, string) {
	// First 4 bytes of the file are the FLAC stream marker: 0x66, 0x4C, 0x61, 0x43
	// It's also the length of all metadata block headers so we'll resue it below.
	headerBuf := make([]byte, 4)
	f.Read(headerBuf)

	if string(headerBuf) != "fLaC" {
		return true, fmt.Sprintf("FATAL: FLAC signature not found in '%s'", f.Name())
	}

	for totalMBH := 0; ; totalMBH++ {
		// Next 4 bytes after the stream marker is the first metadata block header.
		f.Read(headerBuf)

		mbh := new(FLACMetadataBlockHeader)
		error, msg := mbh.FLACParseMetadataBlockHeader(headerBuf)

		if error {
			return true, msg
		}

		error, headerType := LookupHeaderType(mbh.Type)
		block := make([]byte, int(mbh.Length))

		f.Read(block)

		switch headerType {
		case "STREAMINFO":
			if flacmetadata.FLACStreaminfo.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s blocks encountered!\n", headerType)
			}
			sib := new(FLACStreaminfoBlock)
			sib.FLACParseStreaminfoBlock(block)
			flacmetadata.FLACStreaminfo = FLACStreaminfo{mbh, sib, true}

		case "VORBIS_COMMENT":
			if flacmetadata.FLACVorbisComment.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s blocks encountered!\n", headerType)
			}
			vcb := new(FLACVorbisCommentBlock)
			vcb.FLACParseVorbisCommentBlock(block)
			flacmetadata.FLACVorbisComment = FLACVorbisComment{mbh, vcb, true}

		case "PICTURE":
			fpb := new(FLACPictureBlock)
			fpb.FLACParsePictureBlock(block)
			flacmetadata.FLACPictures = append(flacmetadata.FLACPictures, FLACPicture{mbh, fpb, true})

		case "PADDING":
			if flacmetadata.FLACPadding.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s blocks encountered!\n", headerType)
			}
			flacmetadata.FLACPadding = FLACPadding{mbh, nil, true}

		case "APPLICATION":
			if flacmetadata.FLACApplication.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s blocks encountered!\n", headerType)
			}
			fab := new(FLACApplicationBlock)
			fab.FLACParseApplicationBlock(block)
			flacmetadata.FLACApplication = FLACApplication{mbh, fab, true}

		case "SEEKTABLE":
			if flacmetadata.FLACSeektable.IsPopulated {
				return true, fmt.Sprintf("FATAL: Two %s block encountered!\n", headerType)
			}
			spb := FLACParseSeekpointBlock(block)
			flacmetadata.FLACSeektable = FLACSeektable{mbh, spb, true}

		default:
			continue
		}

		if mbh.Last {
			break
		}
	}
	return false, ""
}
