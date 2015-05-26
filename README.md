# beadmachine
beadmachine is a bead pattern creator. Convert any imagine into a suitable color palette pixel by pixel in order to be able to create a matching, beadable pattern. It also shows you a statistic about the used beads.

### Features
- Cross platform
- Uses all available CPU cores to process the image
- Supports gif/jpg/png as input file formats
- Can output a HTML file with detailed info on which bead to use for each pixel
- Color matching based on [CIEDE2000](http://en.wikipedia.org/wiki/Color_difference#CIEDE2000 "")
- Included bead palettes: [Hama](http://www.hama.dk "")
- Optional image resizing
- Image filters to preprocess the input image

### Command-line options:
<dl>
<dt>-i=FILENAME</dt>
  <dd>Filename of image to process.</dd>
<dt>-o=FILENAME.png</dt>
  <dd>Output filename for the converted PNG image.</dd>
<dt>-l=pattern.html</dt>
  <dd>Output filename for a HTML based bead pattern.</dd>
<dt>-p=colors_hama.json</dt>
  <dd>Filename of the bead palette.</dd>
<dt>-w=0</dt>
  <dd>Resize image to width.</dd>
<dt>-h=0</dt>
  <dd>Resize image to height.</dd>
<dt>-x=1</dt>
  <dd>Resize image to width in amount of boards.</dd>
<dt>-y=1</dt>
  <dd>Resize image to height in amount of boards.</dd>
<dt>-b</dt>
  <dd>Make output file look like a beads board.</dd>
<dt>-t</dt>
  <dd>Include translucent colors for the conversion.</dd>
<dt>-f</dt>
  <dd>Include flourescent colors for the conversion.</dd>
<dt>--grey</dt>
  <dd>Convert the image to greyscale.</dd>
<dt>--blur=1.0</dt>
  <dd>Apply blur filter (0.0 - 10.0).</dd>
<dt>--sharpen=1.0</dt>
  <dd>Apply sharpen filter (0.0 - 10.0).</dd>
<dt>--gamma=1.0</dt>
  <dd>Apply gamma correction (0.0 - 10.0).</dd>
<dt>--contrast=1.0</dt>
  <dd>Apply contrast adjustment (-100 - 100).</dd>
<dt>--brightness=1.0</dt>
  <dd>Apply brightness adjustment (-100 - 100).</dd>
</dl>

### Example Usage
To convert the sample yoshi image to Hama bead colors:

```bash
./beadmachine -i examples/yoshi_thinking_in.png -o out.png -l pattern.html
```

<img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_in.png" alt="Yoshi thinking in" height="96" width="84"/> -> <img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_out.png" alt="Yoshi thinking out" height="96" width="84"/>

And will print out a statistic:
```bash
Input image width: 28, height: 32
Bead boards width: 1, height: 1
Beads width: 14 cm, height: 16 cm
Bead colors used: 9
Beads used 'H10 Green': 30
Beads used 'H47 Pastel Green': 72
Beads used 'H1 White': 525
Beads used 'H37 Neon green': 38
Beads used 'H38 Neon orange': 18
Beads used 'H4 Orange': 10
Beads used 'H35 Neon red': 13
Beads used 'H27 Beige': 11
Beads used 'H18 Black': 179
Image processed in 6.0004ms
```

The output of the HTML pattern file will look like this:

<img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/yoshi_thinking_htmlpattern.png" alt="Yoshi HTML pattern"/>

To convert the sample Mona Lisa image to Hama bead colors, resize to a width of 58 pixel and create a bead style output:

```bash
./beadmachine -i examples/mona_lisa_in.jpg -o out.png -w 58 -b --blur 2.75 --contrast 10
```

<img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/mona_lisa_in.jpg" alt="Mona Lisa in" height="461" width="310"/> -> <img src="https://raw.githubusercontent.com/CornelK/beadmachine/master/examples/mona_lisa_out.png" alt="Mona Lisa out" height="461" width="310"/>

And will print out a statistic:
```bash
Input image width: 722, height: 1074
Bead boards width: 2, height: 3
Beads width: 29 cm, height: 43 cm
Output image pixel width: 58, height: 86
Bead colors used: 22
...
```

### Installation
You need to have a [Golang](http://golang.org/doc/install "") environment set up. Install beadmachine:

```bash
go install github.com/CornelK/beadmachine
```

### Todo
- Option to skip color matching
- Mouse over hints in HTML pattern file for multiple same colored beads counts
- Specify different width and height for board dimension
- Perler bead palette
- Support for giving bead stocks as input
- GUI / Webservice
