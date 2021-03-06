package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/karrick/goavro"
)

func usage() {
	executable, err := os.Executable()
	if err != nil {
		executable = os.Args[0]
	}
	base := filepath.Base(executable)
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", base)
	fmt.Fprintf(os.Stderr, "\t%s [-compress null|deflate|snappy] [-count N] from-file to-file\n", base)
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	compress := flag.String("compress", "null", "compression codec ('null', 'deflate', 'snappy'; default: 'null')")
	count := flag.Int("count", 0, "max number of items in each block (zero implies no limit)")
	flag.Parse()

	var compression goavro.Compression
	switch *compress {
	case goavro.CompressionNullLabel:
		// the goavro.Compression zero value specifies the null codec
	case goavro.CompressionDeflateLabel:
		compression = goavro.CompressionDeflate
	case goavro.CompressionSnappyLabel:
		compression = goavro.CompressionSnappy
	default:
		bail(fmt.Errorf("unsupported compression codec: %s", *compress))
	}

	if len(flag.Args()) == 0 {
		usage()
	}

	fromF, err := os.Open(flag.Arg(0))
	if err != nil {
		bail(err)
	}
	defer func(ioc io.Closer) {
		if err := ioc.Close(); err != nil {
			bail(err)
		}
	}(fromF)

	toF, err := os.Create(flag.Arg(1))
	if err != nil {
		bail(err)
	}
	defer func(ioc io.Closer) {
		if err := ioc.Close(); err != nil {
			bail(err)
		}
	}(toF)

	if err := transcode(fromF, toF, compression, *count); err != nil {
		bail(err)
	}
}

func transcode(from io.Reader, to io.Writer, newCodec goavro.Compression, blockCount int) error {
	ocfr, err := goavro.NewOCFReader(from)
	if err != nil {
		return err
	}

	ocfw, err := goavro.NewOCFWriter(goavro.OCFWriterConfig{
		W:           to,
		Schema:      ocfr.Schema(),
		Compression: newCodec,
	})
	if err != nil {
		return err
	}

	var data []interface{}
	for ocfr.Scan() {
		datum, err := ocfr.Read()
		if err != nil {
			return err
		}
		data = append(data, datum)

		if blockCount > 0 && len(data) == blockCount {
			if err := ocfw.Append(data); err != nil {
				return err
			}
			data = nil
		}
	}
	if ocfr.Err(); err != nil {
		return err
	}

	return ocfw.Append(data) // append all remaining items
}

func bail(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
