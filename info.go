/* vile:tabstop=4 */

package main

import (
	"os"
	"fmt"
	"flag"
	"encoding/binary"
	"bytes"
)

var METADATA_BLOCK_HEADER_TYPES = map[uint32] string {
	0:   "STREAMINFO",
	1:   "PADDING",
	2:   "APPLICATION",
	3: 	 "SEEKTABLE",
	4: 	 "VORBIS_COMMENT",
	5: 	 "CUESHEET",
	6:   "PICTURE",
	127: "INVALID",
}

func HeaderType (k uint32) string {
	blkType := METADATA_BLOCK_HEADER_TYPES[k]

	if blkType == "" {
		return "UNKNOWN"
	}
	return blkType
}

type FLACMetadataBlockHeader struct {
	Type   uint32
	Length uint32
	Last	bool
}

func FLACParseMetadataBlockHeader (block uint32) (mbh FLACMetadataBlockHeader) {
	var LASTBLOCK  uint32 = 0x80000000
	var BLOCKTYPE  uint32 = 0x7F000000
	var BLOCKLEN   uint32 = 0x00FFFFFF

	mbh.Type =  (BLOCKTYPE & block)>>24
	mbh.Length = BLOCKLEN & block
	if (LASTBLOCK & block)>>31 == 1 {
		mbh.Last = true
	} else {
		mbh.Last = false
	}
	return mbh
}

type FLACStreaminfoBlock struct {
	minBlockSize	uint16
	maxBlockSize	uint16
	minFrameSize	uint32
	maxFrameSize	uint32
	sampleRate		uint32
	channels		uint8
	bitsPerSample	uint8
	totalSamples	uint64
	md5Signature	string
}
	
func FLACParseStreaminfoBlock (block []byte) (sib FLACStreaminfoBlock) {
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

		minBlockSize = 16 bits
		maxBlockSize + minFrameSize + maxFrameSize = 64 bits
		sampleRate + channels + bitsPerSample + TotalSamples = 64 bits
		md5Signature = 128 bits
	*/

	b := bytes.NewBuffer(block)

	var (
		bigint uint64
		minFSMask uint64 =         0xFFFFFFFFFFFFFFFF
		maxFSMask uint64 =         0x0000000000FFFFFF
		sampRateMask uint64 =      0xFFFFF00000000000
		bitsPerSampMask uint64 =   0x1F000000000
		chMask uint64 =            0xE0000000000
		totSampMask uint64 =       0x0000000FFFFFFFFF
	)

	sib.minBlockSize = binary.BigEndian.Uint16(b.Next(2))

	bigint = binary.BigEndian.Uint64(b.Next(8))
	sib.maxBlockSize = uint16((minFSMask & bigint)>>48)
	sib.minFrameSize = uint32((minFSMask & bigint)>>24)
	sib.maxFrameSize = uint32(maxFSMask & bigint)

	bigint = binary.BigEndian.Uint64(b.Next(8))
	sib.sampleRate = uint32((sampRateMask & bigint)>>44)
	sib.channels = uint8((chMask & bigint)>>41) + 1
	sib.bitsPerSample = uint8((bitsPerSampMask & bigint)>>36) + 1
	sib.totalSamples = bigint & totSampMask

	sib.md5Signature = fmt.Sprintf("%x", b.Next(16))

	return sib
}

type FLACVorbisCommentBlock struct {
	Vendor string
	Comments []string
}

func FLACParseVorbisCommentBlock (block []byte) (vcb FLACVorbisCommentBlock) {
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

	b := bytes.NewBuffer(block)

	var aCommentLen uint32
	var aComment string

	vendorLen := binary.LittleEndian.Uint32(b.Next(4))
	vcb.Vendor = string(b.Next(int(vendorLen)))
	
	totalComments := binary.LittleEndian.Uint32(b.Next(4))

	fmt.Printf("   vendor = %s, totalComments = %d\n", vcb.Vendor, totalComments)

	for totalComments > 0 {
		aCommentLen = binary.LittleEndian.Uint32(b.Next(4))
		aComment = string(b.Next(int(aCommentLen)))
		vcb.Comments = append(vcb.Comments, aComment)
		totalComments--
	}
	return vcb
}

var fileName = flag.String("f", "", "The input file.")
func main() {

	flag.Parse()

	f, err := os.Open(*fileName)
	if err != nil {
		fmt.Printf("FATAL: %s.\n", err)
		os.Exit(-1)
	}
	defer f.Close()

	b := make([]byte, 65536)
	f.Read(b)

	buf := bytes.NewBuffer(b)
	var streamBuf uint32

	if string(buf.Next(4)) != "fLaC" {
		fmt.Printf("FATAL: '%s' is not a FLAC file.\n", *fileName)
		os.Exit(-1)
	}

	lastBlock := false
	// Comments := FLACVorbisCommentBlock

	for lastBlock != true {
		streamBuf = binary.BigEndian.Uint32(buf.Next(4))
		mbh := FLACParseMetadataBlockHeader(streamBuf)
		lastBlock = mbh.Last

		fmt.Printf("Metadata Block: Type = %s (%d) Length = %d Last = %s\n",
				   HeaderType(mbh.Type), mbh.Type, mbh.Length, mbh.Last)	

		if HeaderType(mbh.Type) == "STREAMINFO" {
			StreaminfoBlock := FLACParseStreaminfoBlock(buf.Next(int(mbh.Length)))
			fmt.Println("  ", *fileName, "StreamInfo =", StreaminfoBlock)
		} else if HeaderType(mbh.Type) == "VORBIS_COMMENT" {
			VorbisCommentBlock := FLACParseVorbisCommentBlock(buf.Next(int(mbh.Length)))
			for i, v := range(VorbisCommentBlock.Comments) {
				fmt.Printf("      comment[%d]: %s\n", i, v)
			}
		} else {
			_ = buf.Next(int(mbh.Length))
		}
		fmt.Printf("\n")
	}
}
