package eephttpd

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/eyedeekay/mktorrent"
	"github.com/eyedeekay/sam-forwarder/config"
	"github.com/eyedeekay/sam-forwarder/interface"
	"github.com/eyedeekay/sam-forwarder/tcp"
	"github.com/eyedeekay/samtracker"
	"github.com/gotoolkits/goTorrent/torrent"
	"github.com/radovskyb/watcher"
	"github.com/sosedoff/gitkit"
	"gitlab.com/golang-commonmark/markdown"
)

//EepHttpd is a structure which automatically configured the forwarding of
//a local service to i2p over the SAM API.
type EepHttpd struct {
	*samtracker.SamTracker
	//tracker
	*gitkit.Server
	*watcher.Watcher
	ServeDir string
	up       bool
	magnet   string
	mark     *markdown.Markdown
}

var err error

func (f *EepHttpd) GetType() string {
	return "eephttpd"
}

func (f *EepHttpd) ServeParent() {
	log.Println("Starting eepsite server", f.Base32())
	if err = f.SamTracker.Serve(); err != nil {
		f.Cleanup()
	}
}

//Serve starts the SAM connection and and forwards the local host:port to i2p
func (f *EepHttpd) Serve() error {
	go f.ServeParent()
	f.MakeTorrent()
	if f.Up() {
		log.Println("Starting web server", f.Target())
		if err := http.ListenAndServe(f.Target(), f); err != nil {
			return err
		}
	}
	return nil
}

func (f *EepHttpd) Up() bool {
	return f.up
}

//Close shuts the whole thing down.
func (f *EepHttpd) Close() error {
	return f.SamTracker.Close()
}

func (s *EepHttpd) Load() (samtunnel.SAMTunnel, error) {
	if !s.up {
		log.Println("Started putting tunnel up")
	}
	f, e := s.SamTracker.Load()
	if e != nil {
		return nil, e
	}
	s.SamTracker = f.(*samtracker.SamTracker)

	s.mark = markdown.New(markdown.XHTMLOutput(true))
	s.up = true
	s.Watcher = watcher.New()
	s.Watcher.SetMaxEvents(1)
	s.Watcher.AddRecursive(s.ServeDir)
	go func() {
		for {
			select {
			case event := <-s.Watcher.Event:
				log.Printf("File event %v\n", event)
				err := s.MakeTorrent()
				if err != nil {
					log.Printf("File Watcher Error %e", err)
				}
			case err := <-s.Watcher.Error:
				log.Printf("File Watcher Error %e", err)
			case <-s.Watcher.Closed:
				return
			}
		}
	}()

	go func() {
		s.Watcher.Wait()
		s.Watcher.TriggerEvent(watcher.Create, nil)
		s.Watcher.TriggerEvent(watcher.Remove, nil)
	}()

	log.Println("Finished putting tunnel up")
	return s, nil
}

func (e *EepHttpd) HostName() string {
	return e.Base32()
}

func (e *EepHttpd) MakeTorrent() error {
	t, err := mktorrent.MakeTorrent(e.ServeDir, e.Base32(), "http://"+e.HostName()+"/", "http://"+e.HostName()+"/a", "http://w7tpbzncbcocrqtwwm3nezhnnsw4ozadvi2hmvzdhrqzfxfum7wa.b32.i2p/a")
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(e.ServeDir, e.Base32()) + ".torrent")
	defer f.Close()
	if err != nil {
		return err
	}
	t.Save(f)

	mag := &torrent.MetaInfo{}
	mag.ReadTorrentMetaInfoFile(f)
	e.magnet = "magnet:?xt=urn:btih:" + mag.InfoHash + "=" + "http://" + e.Base32() + "/announce"
	return nil
}

//NewEepHttpd makes a new SAM forwarder with default options, accepts host:port arguments
func NewEepHttpd(host, port string) (*EepHttpd, error) {
	return NewEepHttpdFromOptions(SetHost(host), SetPort(port))
}

func Never(gitkit.Credential, *gitkit.Request) (bool, error) {
	return false, nil
}

//NewEepHttpdFromOptions makes a new SAM forwarder with default options, accepts host:port arguments
func NewEepHttpdFromOptions(opts ...func(*EepHttpd) error) (*EepHttpd, error) {
	var s EepHttpd
	s.SamTracker = &samtracker.SamTracker{
		SAMForwarder: &samforwarder.SAMForwarder{
			Conf: &i2ptunconf.Conf{},
		},
	}
	s.Server = &gitkit.Server{}
	log.Println("Initializing eephttpd")
	for _, o := range opts {
		if err := o(&s); err != nil {
			return nil, err
		}
	}
	s.SamTracker.Init()
	s.SamTracker.Config().SaveFile = true
	pp, _ := strconv.Atoi(s.SamTracker.Config().TargetPort)
	s.SamTracker.Config().TargetPort = strconv.Itoa(pp + 1)
	//	s.tracker.SamTracker = s.SamTracker
	l, e := s.Load()
	s.Server = gitkit.New(gitkit.Config{
		Dir:        s.ServeDir,
		AutoCreate: true,
		Auth:       true, // Turned off by default
	})
	s.Server.AuthFunc = Never
	//log.Println("Options loaded", s.Print())
	if e != nil {
		return nil, e
	}
	return l.(*EepHttpd), nil
}
