package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"text/template"
	"time"
	"unsafe"

	socket "live-go/src"
	"log"
)

const injectedScript = `
<!-- Code injected by go-server -->
<script type="text/javascript">
  // <![CDATA[  <-- For SVG support
  if ("WebSocket" in window) {
  ( function (){

    var address = "{{.protocol}}://" + window.location.hostname + ":{{.port }}";
    var socket = new WebSocket(address);
    socket.onmessage = function (msg) {
      if (msg.data == "reload") window.location.reload();
    };
    console.log("Live reload enabled.");
  } )()
  }
  // ]]>
</script>
    </body>
</html>

  `
const notFoundHTML = `<!doctype html> <html lang="en"> <head> <meta charset="UTF-8" /> <meta name="viewport" content="width=device-width, initial-scale=1.0" /> <title>Not Founc</title> </head> <body><code>Not Found</code></body></html> `

var wsPort = "6969"
var port = "4200"
var dir = "."
var ignoreDirs = []string{"node_modules", ".git", ".expo"}
var watchList = []string{
	".html",
	".htm",
	".xhtml",
	".php",
	".svg",
	".css",
	".sass",
	".scss",
	".json",
	".json",
}

func main() {
	wsPortFlag := flag.String("ws", "6969", "WebSocket connection Port")
	portFlag := flag.String("p", "4200", "Server Port")
	flag.Parse()

	wsPort = *wsPortFlag
	port = *portFlag
	restArgs := flag.Args()

	if len(restArgs) > 0 {
		dirName := restArgs[len(restArgs)-1]
		_, err := os.ReadDir(dirName)
		if err != nil {
			if !os.IsExist(err) {
				fmt.Printf("[x] Dir %s doesn't exist\n", dirName)
			} else {
				fmt.Printf("[x] Something went wrong reading dir %s\n", dirName)
			}
			os.Exit(1)
		}

		dir = dirName
	}

	filenames := make(chan string)
	msgs := make(chan string)

	action := debounce(func() {
		filenames <- "reload"
	}, 100)

	go watchDir(dir, action)

	addFiles(dir)
	fmt.Println("Watching for Changes")

	go http.ListenAndServe(":"+port, nil)
	go socket.Start(msgs, ":"+wsPort)
	i := 0
	for file := range filenames {
		fmt.Println(i, file)
		go func() {
			msgs <- fmt.Sprintf("%s %d", file, i)
		}()
		i++
	}
}

func addFiles(root string) {
	route := "/"
	http.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sanitized := strings.TrimSuffix(path, "/")

		if strings.HasSuffix(path, "/") {
			sanitized = filepath.Join(sanitized, "index.html")
		}

		fileBuff, err := os.ReadFile(filepath.Join(root, sanitized))
		if err != nil {
			w.WriteHeader(404)

			fileBuff := []byte(notFoundHTML)

			val := InjectHtml(fileBuff, path)
			w.Write([]byte(val))
		}

		if strings.HasSuffix(sanitized, ".html") {
			val := InjectHtml(fileBuff, path)
			w.Write([]byte(val))
		} else {
			w.Write(fileBuff)
		}

	})
}

func InjectHtml(buf []byte, filename string) string {
	fileStr := string(buf)
	re := regexp.MustCompile(`(?i)(\s)*</\s*body\s*>\s*</\s*html\s*>\s*`)
	m := map[string]string{
		"port":     wsPort,
		"protocol": "http",
		"filename": filename,
	}
	t := template.Must(template.New("").Parse(injectedScript))
	var htmlBuf strings.Builder
	t.Execute(&htmlBuf, m)

	v := re.ReplaceAllString(fileStr, htmlBuf.String())

	return v
}

func watchDir(dir string, action func()) {
	if slices.Contains(ignoreDirs, dir) {
		return
	}
	fd, err := syscall.InotifyInit()
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(fd)

	wd, err := syscall.InotifyAddWatch(fd, dir,
		syscall.IN_MODIFY|
			syscall.IN_CREATE|
			syscall.IN_DELETE|
			syscall.IN_CLOSE_WRITE|
			syscall.IN_MOVED_TO|
			syscall.IN_MOVE)
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.InotifyRmWatch(fd, uint32(wd))

	ls, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range ls {
		if e.IsDir() {
			go watchDir(filepath.Join(dir, e.Name()), action)
		}
	}

	for {
		buf := make([]byte, 102)
		_, err := syscall.Read(fd, buf)
		if err != nil {
			log.Fatal(err)
		}
		event := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[0]))

		nameBuf := buf[syscall.SizeofInotifyEvent : syscall.SizeofInotifyEvent+int(event.Len)-1]
		name := ""
		for _, b := range nameBuf {
			if b == 0 {
				break
			}
			name += string(b)
		}

		watchable := false
		for _, ext := range watchList {
			if strings.HasSuffix(name, ext) {
				watchable = true
				break
			}
		}

		if !watchable {
			// continue
		}

		fmt.Printf("%s\n", filepath.Join(dir, name))
		action()
	}
}

func debounce(f func(), delay time.Duration) func() {
	var timer *time.Timer

	return func() {
		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(delay, f)
	}
}
