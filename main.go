package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"text/template"
	"unsafe"

	socket "go-live/src"
	"log"
)

const injectedScript = `
<!-- Code injected by go-server -->
<script type="text/javascript">
  // <![CDATA[  <-- For SVG support
  if ("WebSocket" in window) {
    var address = "{{.protocol}}://" + window.location.hostname + ":{{.port }}";
    var socket = new WebSocket(address);
    socket.onmessage = function (msg) {
      if (msg.data == "reload") window.location.reload();
    };
    console.log("Live reload enabled.");
  }
  // ]]>
</script>
    </body>
</html>
  `

var wsPort = "6969"
var port = "4200"
var dir = "."

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
	go watchDir(dir, filenames)

	addFiles(dir)
	fmt.Println("Watching for Changes")

	go http.ListenAndServe(":"+port, nil)
	go socket.Start(msgs, ":"+wsPort)
	for file := range filenames {
		go func() {
			msgs <- file
		}()
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
			w.WriteHeader(500)
		}

		if strings.HasSuffix(path, ".html") {
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

func watchDir(dir string, filename chan<- string) {
	fd, err := syscall.InotifyInit()
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(fd)

	wd, err := syscall.InotifyAddWatch(fd, dir, syscall.IN_MODIFY|syscall.IN_CREATE|syscall.IN_DELETE)
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
			go watchDir(filepath.Join(dir, e.Name()), filename)
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

		fmt.Printf("%s\n", filepath.Join(dir, name))
		filename <- "reload"
	}
}
