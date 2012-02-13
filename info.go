/* vile:tabstop=8 */

/*
TODO:
	FLACParseCuesheetBlock()
	FLACParseCuesheetTrack()
	FLACPraseSeektableBlock()
	FLACParseSeekpoint()
*/

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
)

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

func LookupHeaderType(k uint32) string {
	blkType := METADATA_BLOCK_HEADER_TYPES[k]

	switch blkType {
	case "":
		return "UNKNOWN"
	}
	return blkType
}

func LookupPictureType(k uint32) string {
	t := PictureTypeMap[k]
	switch t {
	case "":
		return "UNKNOWN"
	}
	return t
}

type FLACMetadataBlockHeader struct {
	Type   uint32
	Length uint32
	Last   bool
}

func FLACParseMetadataBlockHeader(block []byte) (mbh FLACMetadataBlockHeader) {
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

	return mbh
}

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

func FLACParseStreaminfoBlock(block []byte) (sib FLACStreaminfoBlock) {
	/* http://flac.sourceforge.net/format.html
	The FLAC STREAMINFO block is structured thus:
	<16>  - Minimum block size (in samples) used in the stream.
	<16>  - Maximum block size (in samples) used in the stream.
	<24>  - Minimum frame size (in bytes) used in the stream. 0 == Implied Unknown
	<24>  - Maximum frame size (in bytes) used in the stream. 0 == Implied Unknown
	<20>  - Sample rate (in Hz). Must be > 0 && < 655350
	<3>   - Number of channels - 1. Why -1?
	<5>   - Bits per sample - 1. Why -1?
	<36>  - Total number of samples in the stream. 0 == Implied Unknown
	<128> - MD5 signature of the unencoded audio data.

	In order to keep everything on powers-of-2 boundaries, reads from the
	block are grouped thus:

		MinBlockSize = 16 bits
		MaxBlockSize + minFrameSize + maxFrameSize = 64 bits
		SampleRate + channels + bitsPerSample + TotalSamples = 64 bits
		md5Signature = 128 bits
	*/

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

	return sib
}

type FLACVorbisCommentBlock struct {
	Vendor        string
	TotalComments uint32
	Comments      []string
}

func FLACParseVorbisCommentBlock(block []byte) (vcb FLACVorbisCommentBlock) {
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

	// vendorLen := int(binary.LittleEndian.Uint32(buf.Next(4)))
	vendorLen := int(binary.LittleEndian.Uint32(buf.Next(4)))
	vcb.Vendor = string(buf.Next(vendorLen))

	vcb.TotalComments = binary.LittleEndian.Uint32(buf.Next(4))

	for tc := vcb.TotalComments; tc > 0; tc-- {
		commentLen := int(binary.LittleEndian.Uint32(buf.Next(4)))
		comment := string(buf.Next(commentLen))
		vcb.Comments = append(vcb.Comments, comment)
	}
	return vcb
}

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

func FLACParsePictureBlock(block []byte) (pb FLACPictureBlock) {
	/*
		<32>	 The picture type according to the ID3v2 APIC frame
		<32>	 The length of the MIME type string in bytes.
		<n*8>	 The MIME type string, in printable ASCII characters 0x20-0x7e. The MIME type may also be --> to signify that the data part is a URL of the picture instead of the picture data itself.
		<32>	 The length of the description string in bytes.
		<n*8>	 The description of the picture, in UTF-8.
		<32>	 The width of the picture in pixels.
		<32>	 The height of the picture in pixels.
		<32>	 The color depth of the picture in bits-per-pixel.
		<32>	 For indexed-color pictures (e.g. GIF), the number of colors used, or 0 for non-indexed pictures.
		<32>	 The length of the picture data in bytes.
		<n*8>	 The binary picture data.
	*/
	buf := bytes.NewBuffer(block)

	pb.PictureType = LookupPictureType(binary.BigEndian.Uint32(buf.Next(4)))

	mimeLen := int(binary.BigEndian.Uint32(buf.Next(4)))
	pb.MimeType = string(buf.Next(mimeLen))

	descLen := int(binary.BigEndian.Uint32(buf.Next(4)))
	if descLen > 0 {
		pb.PictureDescription = string(binary.BigEndian.Uint32(buf.Next(descLen)))
	}
	pb.Width = binary.BigEndian.Uint32(buf.Next(4))
	pb.Height = binary.BigEndian.Uint32(buf.Next(4))
	pb.ColorDepth = binary.BigEndian.Uint32(buf.Next(4))
	pb.NumColors = binary.BigEndian.Uint32(buf.Next(4))
	pb.Length = binary.BigEndian.Uint32(buf.Next(4))
	pb.PictureBlob = hex.Dump(buf.Next(int(pb.Length)))

	return pb
}

type FLACApplicationBlock struct {
	Id   uint32
	Data []byte
}

func FLACParseApplicationBlock(block []byte) (ab FLACApplicationBlock) {
	buf := bytes.NewBuffer(block)

	ab.Id = binary.BigEndian.Uint32(buf.Next(4))
	if buf.Len()%8 != 0 {
		fmt.Println("FATAL: Malformed METADATA_BLOCK_APPLICATION: the data field length is not a mulitple of 8")
		os.Exit(-1)
	}
	ab.Data = buf.Bytes()
	return ab
}

type FLACStreaminfo struct {
	Header FLACMetadataBlockHeader
	Data   FLACStreaminfoBlock
}

type FLACVorbisComment struct {
	Header FLACMetadataBlockHeader
	Data   FLACVorbisCommentBlock
}

type FLACPicture struct {
	Header FLACMetadataBlockHeader
	Data   FLACPictureBlock
}

type FLACApplication struct {
	Header FLACMetadataBlockHeader
	Data   FLACApplicationBlock
}

type FLACPadding struct {
	Header FLACMetadataBlockHeader
	Data   []byte
}

type FLACMetadata struct {
	FLACStreaminfo
	Metadata []interface{}
}

func ReadFLACMetadata(f *os.File) (flacmetadata FLACMetadata) {
	seenStreamInfoBlock := false

	// First 4 bytes of the file are the FLAC stream marker: 0x66, 0x4C, 0x61, 0x43
	// It's also the length of all metadata block headers so we'll resue it below.
	headerBuf := make([]byte, 4)
	f.Read(headerBuf)

	if string(headerBuf) != "fLaC" {
		fmt.Printf("FATAL: '%s' is not a FLAC file.\n", f.Name())
		os.Exit(-1)
	}

	for totalMBH := 0; ; totalMBH++ {
		// Next 4 bytes after the stream marker is the first metadata block header.
		f.Read(headerBuf)
		mbh := FLACParseMetadataBlockHeader(headerBuf)

		headerType := LookupHeaderType(mbh.Type)

		if headerType == "STREAMINFO" && seenStreamInfoBlock {
			fmt.Println("FATAL: Two STREAMINFO blocks encountered!\n")
			os.Exit(-1)
		}

		block := make([]byte, int(mbh.Length))
		f.Read(block)

		switch headerType {
		case "STREAMINFO":
			sib := FLACParseStreaminfoBlock(block)
			flacmetadata.FLACStreaminfo = FLACStreaminfo{mbh, sib}

		case "VORBIS_COMMENT":
			vcb := FLACParseVorbisCommentBlock(block)
			flacmetadata.Metadata = append(flacmetadata.Metadata, FLACVorbisComment{mbh, vcb})

		case "PICTURE":
			fpb := FLACParsePictureBlock(block)
			flacmetadata.Metadata = append(flacmetadata.Metadata, FLACPicture{mbh, fpb})

		case "PADDING": // Don't bother to parse the PADDING block.
			flacmetadata.Metadata = append(flacmetadata.Metadata, FLACPadding{mbh, nil})

		case "APPLICATION":
			fab := FLACParseApplicationBlock(block)
			flacmetadata.Metadata = append(flacmetadata.Metadata, FLACApplication{mbh, fab})

		default:
			continue
		}

		if mbh.Last == true {
			break
		}
	}
	return flacmetadata
}

func PrintFLACMetadataBlockHeader(header FLACMetadataBlockHeader) {
	fmt.Printf("  type: %d (%s)\n", header.Type,
		LookupHeaderType(header.Type))
	fmt.Println("  ls last:", header.Last)
	fmt.Println("  length:", header.Length)
}

func PrintFLACStreaminfoBlockData(data FLACStreaminfoBlock) {
	fmt.Println("  minimum blocksize:", data.MinBlockSize, "samples")
	fmt.Println("  maximum blocksize:", data.MaxBlockSize, "samples")
	fmt.Println("  minimum framesize:", data.MinFrameSize, "bytes")
	fmt.Println("  maximum framesize:", data.MaxFrameSize, "bytes")
	fmt.Println("  sample_rate:", data.SampleRate)
	fmt.Println("  channels:", data.Channels)
	fmt.Println("  bits-per-sample:", data.BitsPerSample)
	fmt.Println("  total samples:", data.TotalSamples)
	fmt.Println("  MD5 signature:", data.MD5Signature)
}

func PrintFLACVorbisCommentData(data FLACVorbisCommentBlock) {
	fmt.Println("  vendor string:", data.Vendor)
	fmt.Println("  comments:", data.TotalComments)
	for i, v := range data.Comments {
		fmt.Printf("    comment[%d]: %s\n", i, v)
	}
}

func PrintFLACPictureData(data FLACPictureBlock) {
	fmt.Println("  type:", data.PictureType)
	fmt.Println("  MIME type:", data.MimeType)
	fmt.Println("  description:", data.PictureDescription)
	fmt.Println("  width:", data.Width)
	fmt.Println("  height:", data.Height)
	fmt.Println("  depth:", data.ColorDepth)
	fmt.Println("  colors:", data.NumColors)
	fmt.Println("  data length:", data.Length)
	for _, l := range strings.Split(data.PictureBlob, "\n") {
		fmt.Println("   ", l)
	}
}

func PrintFLACApplicationData(data FLACApplicationBlock) {
	fmt.Println("  app. id:", string(data.Id))
}

func main() {
	fileName := flag.String("f", "", "The input file.")
	flag.Parse()

	f, err := os.Open(*fileName)
	if err != nil {
		fmt.Println("FATAL: %s", err)
		os.Exit(-1)
	}
	defer f.Close()

	metadata := ReadFLACMetadata(f)

	fmt.Println("METADATA block #0")
	PrintFLACMetadataBlockHeader(metadata.FLACStreaminfo.Header)
	PrintFLACStreaminfoBlockData(metadata.FLACStreaminfo.Data)

	for i, b := 0, 1; i < len(metadata.Metadata); i++ {
		fmt.Printf("METADATA block #%d\n", b)
		b++

		switch block := metadata.Metadata[i].(type) {
		case FLACVorbisComment:
			PrintFLACMetadataBlockHeader(block.Header)
			PrintFLACVorbisCommentData(block.Data)

		case FLACPicture:
			PrintFLACMetadataBlockHeader(block.Header)
			PrintFLACPictureData(block.Data)

		case FLACApplication:
			PrintFLACMetadataBlockHeader(block.Header)
			PrintFLACApplicationData(block.Data)

		case FLACPadding:
			PrintFLACMetadataBlockHeader(block.Header)

		default:
			fmt.Printf("This block is of type %T\n", block)
		}
	}
	os.Exit(0)
}
