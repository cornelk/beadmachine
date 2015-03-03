# beadmachine
beadmachine is a bead pattern creator. Convert any image to a bead pattern that matches the bead colors for every pixel with the best matching bead color. It also generates a statistic about the used beads.

### Features
- Cross platform
- Uses all available cores to process the image
- Supports gif/jpg/png as input file formats
- Color matching based on [CIEDE2000](http://en.wikipedia.org/wiki/Color_difference#CIEDE2000 "")
- Included bead palettes: [Hama](http://www.hama.dk "")

### Command-line options:
<dl>
<dt>-i=FILENAME</dt>
  <dd>Filename of image to process.</dd>
<dt>-o=FILENAME.png</dt>
  <dd>Output filename for the converted PNG image.</dd>
<dt>-p=colors_hama.json</dt>
  <dd>Filename of the bead palette.</dd>
</dl>

### Example Usage
To convert the sample yoshi image to Hama bead colors:

```bash
./beadmachine -i examples/yoshi_thinking_in.png -o out.png
```

<img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_in.png" alt="Yoshi thinking in" style="width: 80px;"/> converts to <img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_out.png" alt="Yoshi thinking in" style="width: 80px;"/>

### Installation
You need to have a [Golang](http://golang.org/doc/install "") environment set up first. Download beadmachine:

```bash
go get github.com/CornelK/beadmachine
```

Change to the directory inside your GOPATH and build the application:

```bash
cd $GOPATH/src/github.com/CornelK/beatmachine
go build
```

### Todo
- Image resizing
- Perler palette
- Support for giving bead stocks as input
- GUI
