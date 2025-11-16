# FySlide

A Fyne-based Image Browser and Slideshow application with powerful tagging and filtering capabilities.

## Features

*   **Image Browsing**: Navigate through images with keyboard shortcuts (arrows, home, end, etc.) or UI buttons.
*   **Slideshow**: View images in an automated slideshow with configurable timing.
*   **Random Mode**: Shuffle the image viewing order.
*   **Zoom & Pan**: Zoom in and out of images with the mouse wheel or keys, and pan by dragging.
*   **Image Manipulation**: Rotate the current image clockwise.
*   **Advanced Scaling**: Choose between Nearest Neighbor, Bilinear, and Bicubic scaling algorithms for optimal image quality.
*   **Thumbnail Browser**: A collapsible thumbnail strip for quick visual navigation.
*   **Tagging**: Add and remove tags for individual images or entire directories.
*   **Filtering**: Filter the image list by one or more tags.
*   **Tag Management**: A dedicated view to see all tags, their usage counts, and remove tags globally.
*   **Persistent Database**: Tags are stored in a local BoltDB database.
*   **Theming**: Supports both light and dark themes.

## Screenshots

![FySlide Screenshot](./assets/screenshot.png)

*The main window of FySlide showing an image, the thumbnail browser, and the information panel.*

## Building from Source

### Prerequisites

*   Go (version 1.18 or later)
*   Fyne CLI tool (`go install fyne.io/fyne/v2/cmd/fyne@latest`)
*   A C compiler (like GCC) for Fyne dependencies.

### Build Command

From the project root directory, run:

```bash
go build -ldflags="-s -w" -o fyslide main.go
```

## Command-line Flags

The application's behavior can be modified with the following flags. The final argument should be the directory to scan.

| Flag                 | Default | Description                                            |
| -------------------- | ------- | ------------------------------------------------------ |
| `-load-db`           | `false` | Pre-load known image paths from database on startup.   |
| `-slideshow-interval`| `3.0`   | Slideshow image display interval in seconds.           |
| `-skip-count`        | `20`    | Number of images to skip with PageUp/PageDown.         |

Example:
```bash
./fyslide -slideshow-interval=5.0 /path/to/your/images
```

## Keyboard Shortcuts

| Description                   | Shortcut                |
| ----------------------------- | ----------------------- |
| Quit Application              | `Ctrl+Q` or `Q`         |
| Next Image                    | `Arrow Right`           |
| Previous Image                | `Arrow Left`            |
| Skip Images Forward           | `Page Down`             |
| Skip Images Back              | `Page Up`               |
| First Image                   | `Home`                  |
| Last Image                    | `End`                   |
| Next Untagged Image           | `Ctrl+Arrow Right`      |
| Previous Untagged Image       | `Ctrl+Arrow Left`       |
| Toggle Play/Pause Slideshow   | `P` or `Space`          |
| Rotate Image Clockwise        | `R`                     |
| Delete Current Image          | `Delete`                |
| Close Dialog/Overlay          | `Esc`                   |
| Zoom In Image                 | `+`                     |
| Zoom Out Image                | `-`                     |
| Reset Image Zoom/Pan          | `0` or `Insert`         |
| Pan Image Up                  | `Arrow Up`              |
| Pan Image Down                | `Arrow Down`            |

## Command-line Interface (CLI)

A companion CLI tool is available for managing tags from the terminal. Build it with:
```bash
go build -o fyslide-cli ./cmd/fyslide-cli
```
Run `./fyslide-cli --help` for a full list of commands, such as adding, removing, and listing tags.

## Database

FySlide stores all tag information in a file named `fyslide_tags.db`. This file is located in the standard user configuration directory for your operating system (e.g., `~/.config/fyslide/` on Linux).

## Acknowledgements

Heavily leveraged from myfyneapplication.

## Environment Variables

The application can read certain values from the environment. Setting these variables can be done by the usual methods for the OS on which the application is running.

The variables that can be set are:

**FYNE_THEME**: This specifies whether to override the default OS theme with either "dark" or "light" theme variants.

## Folder Structure

The source code tries to follow the standard Go structure for laying out source code. More information on that structure can be found here: Golang Standards -- Project Layout.

**internal**: Contains internal packages that are used by the application, but not intended to be exported as standalone packages.

**assets**: Assets for the application such as PNG files, etc.

## Bundling Assets

Go allows for the bundling of assets like images into the binary itself. This can be accomplished with the command:

```
fyne bundle --output <filename>.go <imagefile.ext>
```

For this project, an example might be:

```
fyne bundle --package ui --output internal/ui/bundle.go assets/icon.png
```

This process has been automated using **go:generate** headers in main.go for this application. To regenerate the bundle.go file if an asset changes, use:

```
go generate
```

from the project root folder. Note that assets can be added to the bundle with the --append parameter:

```
fyne bundle --package ui --output internal/ui/bundle.go --append anotherimage.png
```
