# FySlide #

Fyne Image Browser and Slide Show.

# Acknowledgements #
Heavily leveraged from (https://github.com/mrunion/myfyneapplication)[https://github.com/mrunion/myfyneapplication]

## Environment Variables ##

The application can read certain values from the environment. Setting these variables can be done by the usual methods for the OS on which the application is running.

The variables that can be set are:

**FYNE_THEME**: This specifies wether to override the default OS theme with either "dark" or "light" theme variants.

## Folder Structure ##

The source code tries to follow the standard Go structure for laying out source code. More information on that structure can be found here [Golang Standards -- Project Layout](https://github.com/golang-standards/project-layout).

**internal**: Contains internal packages that are used by the application, but not intended to be exported as standalone packages.

**assets**: Assets for the application such as PNG files, etc.

## Bundling Assets ##

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
