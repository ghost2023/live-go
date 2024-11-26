# Live Go

Live Server implementation in Golang for better perfomance and low memory footprint. It has no external dependencies. Currently only works on linux because it relies on system calls that may or maynot work on other platforms.

## Installation

Working on deploying the executable but anyone who is interested can class and build it.
I will try to make the process more streamline

## Usage from command line

Issue the command `live-go` in your project's directory. Alternatively you can add the path to serve as a command line parameter.

This will automatically launch the default browser. When you make a change to any file, the browser will reload the page - unless it was a CSS file in which case the changes are applied without a reload.

Command line parameters:

- `--p=NUMBER` - select port to use, default: 4200
- `--ws=NUMBER` - select port to use the websocket connection, default: 6969
- `--help | -h` - display terse usage hint and exit

## Troubleshooting

- Before you test it keep in mind this will work best on linux.
- No reload on changes
  - Open your browser's console: there should be a message at the top stating that live reload is enabled. Note that you will need a browser that supports WebSockets. If there are errors, deal with them. If it's still not working, file an issue.

## Inspiration

- [live-server](https://github.com/tapio/live-server)
