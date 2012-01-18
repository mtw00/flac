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

func HeaderType(k uint32) string {
	blkType := METADATA_BLOCK_HEADER_TYPES[k]

	if blkType == "" {
		return "UNKNOWN"
	}
	return blkType
}

type FLACMetadataBlockHeader struct {
	Type   uint32
	Length uint32
	Last   bool
}

func FLACParseMetadataBlockHeader(block uint32) (mbh FLACMetadataBlockHeader) {
	var LASTBLOCK uint32 = 0x80000000
	var BLOCKTYPE uint32 = 0x7F000000
	var BLOCKLEN uint32 = 0x00FFFFFF

	mbh.Type = (BLOCKTYPE & block) >> 24
	mbh.Length = BLOCKLEN & block
	if (LASTBLOCK&block)>>31 == 1 {
		mbh.Last = true
	} else {
		mbh.Last = false
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

	b := bytes.NewBuffer(block)

	var (
		bigint          uint64
		minFSMask       uint64 = 0xFFFFFFFFFFFFFFFF
		maxFSMask       uint64 = 0xFFFFFF
		sampRateMask    uint64 = 0xFFFFF00000000000
		bitsPerSampMask uint64 = 0x1F000000000
		chMask          uint64 = 0xE0000000000
		totSampMask     uint64 = 0xFFFFFFFFF
	)

	sib.MinBlockSize = binary.BigEndian.Uint16(b.Next(2))

	bigint = binary.BigEndian.Uint64(b.Next(8))
	sib.MaxBlockSize = uint16((minFSMask & bigint) >> 48)
	sib.MinFrameSize = uint32((minFSMask & bigint) >> 24)
	sib.MaxFrameSize = uint32(maxFSMask & bigint)

	bigint = binary.BigEndian.Uint64(b.Next(8))
	sib.SampleRate = uint32((sampRateMask & bigint) >> 44)
	sib.Channels = uint8((chMask&bigint)>>41) + 1
	sib.BitsPerSample = uint8((bitsPerSampMask&bigint)>>36) + 1
	sib.TotalSamples = bigint & totSampMask

	sib.MD5Signature = fmt.Sprintf("%x", b.Next(16))

	return sib
}

type FLACVorbisCommentBlock struct {
	Vendor        string
	TotalComments uint32
	Comments      []string
}

func FLACParseVorbisCommentBlock(block []byte) (vcb FLACVorbisCommentBlock) {
	/*
		http://www.xiph.org/vorbis/doc/v-comment.html
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

	aComment := ""
	b := bytes.NewBuffer(block)

	vendorLen := int(binary.LittleEndian.Uint32(b.Next(4)))
	vcb.Vendor = string(b.Next(vendorLen))
	
	vcb.TotalComments = binary.LittleEndian.Uint32(b.Next(4))

	for tc := vcb.TotalComments; tc > 0; tc-- {
		aComment = string(b.Next(int(binary.LittleEndian.Uint32(b.Next(4)))))
		vcb.Comments = append(vcb.Comments, aComment)
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

func LookupPictureType(k uint32) string {
	return PictureTypeMap[k]
}

func FLACParsePictureBlock(block []byte) (pb FLACPictureBlock) {
	/*
		<32>	 The picture type according to the ID3v2 APIC frame:
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
	b := bytes.NewBuffer(block)

	pb.PictureType = LookupPictureType(binary.BigEndian.Uint32(b.Next(4)))

	mimeLen := int(binary.BigEndian.Uint32(b.Next(4)))
	pb.MimeType = string(b.Next(mimeLen))

	descLen := int(binary.BigEndian.Uint32(b.Next(4)))
	if descLen > 0 {
		pb.PictureDescription = string(binary.BigEndian.Uint32(b.Next(descLen)))
	}
	pb.Width = binary.BigEndian.Uint32(b.Next(4))
	pb.Height = binary.BigEndian.Uint32(b.Next(4))
	pb.ColorDepth = binary.BigEndian.Uint32(b.Next(4))
	pb.NumColors = binary.BigEndian.Uint32(b.Next(4))
	pb.Length = binary.BigEndian.Uint32(b.Next(4))
	pb.PictureBlob = hex.Dump(b.Next(int(pb.Length)))

	return pb
}

type FLACStreaminfo struct {
	StreaminfoHeader FLACMetadataBlockHeader
	StreaminfoData   FLACStreaminfoBlock
}

type FLACVorbisComment struct {
	VorbisCommentHeader FLACMetadataBlockHeader
	VorbisCommentData   FLACVorbisCommentBlock
}

type FLACPicture struct {
	PictureHeader FLACMetadataBlockHeader
	PictureData   FLACPictureBlock
}

type FLACMetadata struct {
	FLACMetadataBlockHeader
	FLACStreaminfoBlock
	FLACVorbisCommentBlock
	FLACPictureBlock
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

	b := make([]byte, 65536)
	f.Read(b)
	buf := bytes.NewBuffer(b)

	var metadata []interface{}

	// First 4 bytes of the file are the FLAC stream marker: 0x66, 0x4C, 0x61, 0x43
	if string(buf.Next(4)) != "fLaC" {
		fmt.Printf("FATAL: '%s' is not a FLAC file.\n", *fileName)
		os.Exit(-1)
	}

	for totalMBH := 0; ; totalMBH++ {
		// Next 4 bytes after the stream marker is the first metadata block header.
		mbh := FLACParseMetadataBlockHeader(binary.BigEndian.Uint32(buf.Next(4)))

		switch HeaderType(mbh.Type) {
		case "STREAMINFO":
			sib := FLACParseStreaminfoBlock(buf.Next(int(mbh.Length)))
			metadata = append(metadata, mbh)
			metadata = append(metadata, sib)

		case "VORBIS_COMMENT":
			vcb := FLACParseVorbisCommentBlock(buf.Next(int(mbh.Length)))
			metadata = append(metadata, mbh)
			metadata = append(metadata, vcb)

		case "PICTURE":
			pb := FLACParsePictureBlock(buf.Next(int(mbh.Length)))
			metadata = append(metadata, mbh)
			metadata = append(metadata, pb)

		case "PADDING":		// Don't bother to parse the PADDING block.
			metadata = append(metadata, mbh)

		default:
			_ = buf.Next(int(mbh.Length))
		}

		if mbh.Last == true {
			break 
		}
	}
	for i, j := 0, 0; i < len(metadata); i++ {
		switch d := metadata[i].(type) {
		case FLACMetadataBlockHeader:
			fmt.Printf("METADATA block #%d\n", j); j++
			fmt.Printf("  type: %d (%s)\n", d.Type, HeaderType(d.Type))
			fmt.Println("  ls last:", d.Last)
			fmt.Println("  length:", d.Length)

		case FLACStreaminfoBlock:
			fmt.Println("  minimum blocksize:", d.MinBlockSize, "samples")
			fmt.Println("  maximum blocksize:", d.MaxBlockSize, "samples")
			fmt.Println("  minimum framesize:", d.MinFrameSize, "bytes")
			fmt.Println("  maximum framesize:", d.MaxFrameSize, "bytes")
			fmt.Println("  sample_rate:", d.SampleRate)
			fmt.Println("  channels:", d.Channels)
			fmt.Println("  bits-per-sample:", d.BitsPerSample)
			fmt.Println("  total samples:", d.TotalSamples)
			fmt.Println("  MD5 signature:", d.MD5Signature)

		case FLACVorbisCommentBlock:
			fmt.Println("  vendor string:", d.Vendor)
			fmt.Println("  comments:", d.TotalComments)
			for i, v := range d.Comments {
				fmt.Printf("    comment[%d]: %s\n", i, v)
			}

		case FLACPictureBlock:
			fmt.Println("  type:", d.PictureType)
			fmt.Println("  MIME type:", d.MimeType)
			fmt.Println("  description:", d.PictureDescription)
			fmt.Println("  width:", d.Width)
			fmt.Println("  height:", d.Height)
			fmt.Println("  depth:", d.ColorDepth)
			fmt.Println("  colors:", d.NumColors)
			fmt.Println("  data length:", d.Length)
			for _, l := range strings.Split(d.PictureBlob, "\n") {
				fmt.Println("   ", l)
			}

		default:
			fmt.Println(metadata[i])
		}
	}
}
