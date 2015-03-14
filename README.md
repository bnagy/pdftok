Crashwalk
=======

## Documentation

Unless you are a hacker or a weirdo this is not what you are looking for.

```bash
$ ./pdftok /Users/ben/scratch/pdfs/test_pdfa_gray.pdf | head -10
main.item{typ:11, pos:0, val:"%PDF-1.3"}
main.item{typ:3, pos:8, val:"\n"}
main.item{typ:11, pos:9, val:"%\xe2\xe3\xcf\xd3"}
main.item{typ:3, pos:14, val:"\r\n"}
main.item{typ:2, pos:16, val:"3"}
main.item{typ:3, pos:17, val:" "}
main.item{typ:2, pos:18, val:"0"}
main.item{typ:3, pos:19, val:" "}
main.item{typ:15, pos:20, val:"obj"}
main.item{typ:3, pos:23, val:"\n"}
[...]
```

Token types (EOF -> 1, itemNumber -> 2 etc):
```go
const (
  itemError itemType = iota // error occurred; value is text of error
  itemEOF
  itemNumber    // PDF Number 7.3.3
  itemSpace     // run of space characters 7.2.2 Table 1
  itemLeftDict  // Just the << token
  itemRightDict // >> token
  itemLeftArray
  itemRightArray
  itemStreamBody // raw contents of a stream
  itemString     // PDF Literal String 7.3.4.2
  itemHexString  // PDF Hex String 7.3.4.3
  itemComment    // 7.2.3
  itemName       // PDF Name Object 7.3.5
  itemWord       // catchall for an unrecognised blob of alnums
  // Keywords appear after all the rest.
  itemKeyword // used only to delimit the keywords
  itemObj     // just the obj and endobj markers
  itemEndObj
  itemStream // just the markers
  itemEndStream
  itemTrailer
  itemXref
  itemStartXref
  itemTrue  // not really keywords, they're actually types of
  itemFalse // PDF Basic Object, but this is cleaner 7.3.2
  itemNull
)
```

## Installation

You should follow the [instructions](https://golang.org/doc/install) to
install Go, if you haven't already done so.

Now, install pdftok:
```bash
$ go get -u github.com/bnagy/pdftok
```

## TODO

I lexed a bunch of the Adobe Engineering test files (eg from [here](http://acroeng.adobe.com/wp/?page_id=10) and put the Literal Name tokens in [toks.txt](toks.txt). These should be further curated (by hand) and used to augment a PDF token dictionary. That dictionary would then be useful for fuzzing practitioners.

## Contributing

Fork and send a pull request.

Report issues.

## License & Acknowledgements

BSD style, see LICENSE file for details.

Code heavily based on [this](http://cuddle.googlecode.com/hg/talk/lex.html) awesome talk by Rob Pike, and its implementation in the Go standard library in the `text/template` package.

