package graph

import (
	"net/http"
	"net/url"
	"strings"
//	"strconv"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/archive"
    "github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

func (s *TagStore) CmdImport(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 2 && n != 3 {
		return job.Errorf("Usage: %s SRC REPO [TAG]", job.Name)
	}
	var (
		src     = job.Args[0]
		repo    = job.Args[1]
		tag     string
		sf      = utils.NewStreamFormatter(job.GetenvBool("json"))
		archive archive.ArchiveReader
		resp    *http.Response
	)
	if len(job.Args) > 2 {
		tag = job.Args[2]
	}

	if src == "-" {
		archive = job.Stdin
	} else {
		u, err := url.Parse(src)
		if err != nil {
			return job.Error(err)
		}
		if u.Scheme == "" {
			u.Scheme = "http"
			u.Host = src
			u.Path = ""
		}
		job.Stdout.Write(sf.FormatStatus("", "Downloading from %s", u))
		resp, err = utils.Download(u.String())
		if err != nil {
			return job.Error(err)
		}
		progressReader := utils.ProgressReader(resp.Body, int(resp.ContentLength), job.Stdout, sf, true, "", "Importing")
		defer progressReader.Close()
		archive = progressReader
	}
	var (
		changes = job.GetenvList("changes")
		config runconfig.Config
		/*allowedChanges = map[string]int{
		"env": 0,
		"cmd": 0,
		"maintainer": 0,
		}*/
	)
	if len(changes) > 0 {
		for _, c := range strings.Split(job.Getenv("changes"), "\n") {
			if strings.HasPrefix(c, "ENV"){
				config.Env = append(config.Env, strings.Trim(c,"ENV "))
			}

			if strings.HasPrefix(c, "CMD") {
				config.Cmd = append(config.Cmd, strings.Trim(c,"CMD "))
			}
			if strings.HasPrefix(c, "MAINTAINER") {
				config.Maintainer = c[strings.Index(c," ")+1:len(c)]
			}
		}
	}

	img, err := s.graph.Create(archive, "", "", "Imported from "+src, "", nil, &config)
	if err != nil {
		return job.Error(err)
	}
	// Optionally register the image at REPO/TAG
	if repo != "" {
		if err := s.Set(repo, tag, img.ID, true); err != nil {
			return job.Error(err)
		}
	}
	job.Stdout.Write(sf.FormatStatus("", img.ID))
	return engine.StatusOK
}
