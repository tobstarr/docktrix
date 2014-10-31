package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/dynport/metrix/metrix"
)

var logger = log.New(os.Stderr, "", 0)

func main() {
	if err := run(); err != nil {
		logger.Fatal(err)
	}
}

func run() error {
	logger.Printf("running")
	s := &Server{}
	flag.Parse()
	args := flag.Args()
	logger.Printf("using args %q", args)
	if len(args) > 0 {
		s.Cmd = args[0]
		if len(args) > 1 {
			s.Args = args[1:]
		}
		if err := s.Run(); err != nil {
			return err
		}
	}
	f := "fixtures/meminfo"
	if _, err := os.Stat(f); err == nil {
		s.ProcRoot = "fixtures"
	}
	return http.ListenAndServe(":1235", s)
}

type Server struct {
	Cmd         string
	Args        []string
	LogToBuffer bool

	c        *exec.Cmd
	out      *bytes.Buffer
	err      *bytes.Buffer
	ProcRoot string
}

type Status struct {
	Meminfo   *metrix.Meminfo   `json:"mem,omitempty"`
	ProcStat  *metrix.ProcStat  `json:"proc_stat,omitempty"`
	Stat      *metrix.Stat      `json:"stat,omitempty"`
	Load      *metrix.LoadAvg   `json:"load,omitempty"`
	OpenFiles *metrix.OpenFiles `json:"open_files,omitempty"`

	procRoot string
}

func (s *Status) loadOpenFiles() error {
	c := exec.Command("lsof", "-F")
	c.Stderr = os.Stderr
	o, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	defer o.Close()
	if err := c.Start(); err != nil {
		return err
	}
	s.OpenFiles = &metrix.OpenFiles{}
	return s.OpenFiles.Load(o)
}

func (s *Status) loadStat() error {
	s.Stat = &metrix.Stat{}
	f, err := os.Open(s.procRoot + "/stat")
	if err != nil {
		return err
	}
	defer f.Close()
	return s.Stat.Load(f)
}

func (s *Status) loadLoadAvg() error {
	s.Load = &metrix.LoadAvg{}
	f, err := os.Open(s.procRoot + "/loadavg")
	if err != nil {
		return err
	}
	defer f.Close()
	return s.Load.Load(f)
}

func (s *Status) loadMeminfo() error {
	s.Meminfo = &metrix.Meminfo{}
	f, err := os.Open(s.procRoot + "/meminfo")
	if err != nil {
		return err
	}
	defer f.Close()
	return s.Meminfo.Load(f)
}

func wrapError(s string, f func() error) func() error {
	return func() error {
		if err := f(); err != nil {
			return errors.New(s + ": " + err.Error())
		}
		return nil
	}
}

func (d *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := func() error {
		if d.ProcRoot == "" {
			d.ProcRoot = "/proc"
		}
		s := &Status{procRoot: d.ProcRoot}
		funcs := []func() error{
			wrapError("loadMeminfo", s.loadMeminfo),
			wrapError("loadStat", s.loadStat),
			wrapError("loadLoad", s.loadLoadAvg),
			//wrapError("loadOpenFiles", s.loadOpenFiles),
		}
		for _, f := range funcs {
			if err := f(); err != nil {
				logger.Printf("ERROR: %s", err)
			}
		}
		if d.c.Process.Pid != 0 {
			err := func() error {
				f, err := os.Open(fmt.Sprintf("/proc/%d/stat", d.c.Process.Pid))
				if err != nil {
					return err
				}
				defer f.Close()
				s.ProcStat = &metrix.ProcStat{}
				return s.ProcStat.Load(f)
			}()
			if err != nil {
				logger.Printf("ERROR: %q", err)
			}
		}
		w.Header().Add("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(s)
	}()
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (d *Server) Run() error {
	d.c = exec.Command(d.Cmd, d.Args...)
	if d.LogToBuffer {
		d.out = &bytes.Buffer{}
		d.err = &bytes.Buffer{}
		d.c.Stdout = d.out
		d.c.Stderr = d.err
	} else {
		d.c.Stdout = os.Stdout
		d.c.Stderr = os.Stderr
	}
	err := d.c.Start()
	if err != nil {
		return err
	}
	logger.Printf("started with pid %d", d.c.Process.Pid)
	return nil
}

func (d *Server) Close() error {
	if d.c != nil {
		return d.c.Process.Kill()
	}
	return nil
}
