package test_utils

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	http_server "github.com/kluctl/kluctl/v2/pkg/git/http-server"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type GitServer struct {
	t *testing.T

	baseDir string

	gitServer     *http_server.Server
	gitHttpServer *http.Server
	gitServerPort int
}

func NewGitServer(t *testing.T) *GitServer {
	p := &GitServer{
		t: t,
	}

	baseDir, err := os.MkdirTemp(os.TempDir(), "kluctl-tests-")
	if err != nil {
		p.t.Fatal(err)
	}
	p.baseDir = baseDir

	p.initGitServer()

	t.Cleanup(func() {
		p.Cleanup()
	})

	return p
}

func (p *GitServer) initGitServer() {
	p.gitServer = http_server.New(p.baseDir)

	p.gitHttpServer = &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: p.gitServer,
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	a := ln.Addr().(*net.TCPAddr)
	p.gitServerPort = a.Port

	go func() {
		_ = p.gitHttpServer.Serve(ln)
	}()
}

func (p *GitServer) Cleanup() {
	if p.gitHttpServer != nil {
		_ = p.gitHttpServer.Shutdown(context.Background())
		p.gitHttpServer = nil
		p.gitServer = nil
	}

	if p.baseDir == "" {
		return
	}
	_ = os.RemoveAll(p.baseDir)
	p.baseDir = ""
}

func (p *GitServer) GitInit(repo string) {
	dir := p.LocalRepoDir(repo)

	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		p.t.Fatal(err)
	}

	r, err := git.PlainInit(dir, false)
	if err != nil {
		p.t.Fatal(err)
	}
	config, err := r.Config()
	if err != nil {
		p.t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}

	config.User.Name = "Test User"
	config.User.Email = "no@mail.com"
	config.Author = config.User
	config.Committer = config.User
	err = r.SetConfig(config)
	if err != nil {
		p.t.Fatal(err)
	}
	err = utils.Touch(filepath.Join(dir, ".dummy"))
	if err != nil {
		p.t.Fatal(err)
	}
	_, err = wt.Add(".dummy")
	if err != nil {
		p.t.Fatal(err)
	}
	_, err = wt.Commit("initial", &git.CommitOptions{})
	if err != nil {
		p.t.Fatal(err)
	}
}

func (p *GitServer) CommitFiles(repo string, add []string, all bool, message string) {
	r, err := git.PlainOpen(p.LocalRepoDir(repo))
	if err != nil {
		p.t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}
	for _, a := range add {
		_, err = wt.Add(a)
		if err != nil {
			p.t.Fatal(err)
		}
	}
	_, err = wt.Commit(message, &git.CommitOptions{
		All: all,
	})
	if err != nil {
		p.t.Fatal(err)
	}
}

func (p *GitServer) CommitYaml(repo string, pth string, message string, y *uo.UnstructuredObject) {
	fullPath := filepath.Join(p.LocalRepoDir(repo), pth)

	dir, _ := filepath.Split(fullPath)
	if dir != "" {
		err := os.MkdirAll(dir, 0o700)
		if err != nil {
			panic(err)
		}
	}

	err := yaml.WriteYamlFile(fullPath, y)
	if err != nil {
		p.t.Fatal(err)
	}
	if message == "" {
		message = fmt.Sprintf("update %s", filepath.Join(repo, pth))
	}
	p.CommitFiles(repo, []string{pth}, false, message)
}

func (p *GitServer) UpdateFile(repo string, pth string, update func(f string) (string, error), message string) {
	fullPath := filepath.Join(p.LocalRepoDir(repo), pth)
	f := ""
	if utils.Exists(fullPath) {
		b, err := os.ReadFile(fullPath)
		if err != nil {
			p.t.Fatal(err)
		}
		f = string(b)
	}

	newF, err := update(f)
	if err != nil {
		p.t.Fatal(err)
	}

	if f == newF {
		return
	}
	err = os.WriteFile(fullPath, []byte(newF), 0o600)
	if err != nil {
		p.t.Fatal(err)
	}
	p.CommitFiles(repo, []string{pth}, false, message)
}

func (p *GitServer) UpdateYaml(repo string, pth string, update func(o *uo.UnstructuredObject) error, message string) {
	fullPath := filepath.Join(p.LocalRepoDir(repo), pth)

	o := uo.New()
	if utils.Exists(fullPath) {
		err := yaml.ReadYamlFile(fullPath, o)
		if err != nil {
			p.t.Fatal(err)
		}
	}
	orig := o.Clone()
	err := update(o)
	if err != nil {
		p.t.Fatal(err)
	}
	if reflect.DeepEqual(o, orig) {
		return
	}
	p.CommitYaml(repo, pth, message, o)
}

func (p *GitServer) convertInterfaceToList(x interface{}) []interface{} {
	var ret []interface{}
	if l, ok := x.([]interface{}); ok {
		return l
	}
	if l, ok := x.([]*uo.UnstructuredObject); ok {
		for _, y := range l {
			ret = append(ret, y)
		}
		return ret
	}
	if l, ok := x.([]map[string]interface{}); ok {
		for _, y := range l {
			ret = append(ret, y)
		}
		return ret
	}
	return []interface{}{x}
}

func (p *GitServer) LocalGitUrl(repo string) string {
	return fmt.Sprintf("http://localhost:%d/%s/.git", p.gitServerPort, repo)
}

func (p *GitServer) LocalRepoDir(repo string) string {
	return filepath.Join(p.baseDir, repo)
}

func (p *GitServer) GetWorktree(repo string) *git.Worktree {
	r, err := git.PlainOpen(p.LocalRepoDir(repo))
	if err != nil {
		p.t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}
	return wt
}
