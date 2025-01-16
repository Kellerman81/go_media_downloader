package sabnzbd

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Source: https://github.com/mrobinsn/go-sabnzbd - fixed:add category.
func (s *Sabnzbd) Version() (version string, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.SetMode("version")
	r := &versionResponse{}
	err = u.CallJSON(r)
	return r.Version, err
}

func (s *Sabnzbd) Auth() (auth string, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.SetMode("auth")
	r := &authResponse{}
	err = u.CallJSON(r)
	return r.Auth, err
}

func (s *Sabnzbd) SimpleQueue() (r *SimpleQueueResponse, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("qstatus")
	r = &SimpleQueueResponse{}
	err = u.CallJSON(r)
	return r, err
}

func (s *Sabnzbd) AdvancedQueue(start, limit int) (r *AdvancedQueueResponse, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("queue")
	u.v.Set("start", strconv.Itoa(start))
	u.v.Set("limit", strconv.Itoa(limit))
	r = &AdvancedQueueResponse{}
	err = u.CallJSON(r)
	return r, err
}

func (s *Sabnzbd) History(start, limit int) (r *HistoryResponse, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("history")
	u.v.Set("start", strconv.Itoa(start))
	u.v.Set("limit", strconv.Itoa(limit))
	r = &HistoryResponse{}
	err = u.CallJSON(r)
	return r, err
}

func (s *Sabnzbd) Warnings() (warnings []string, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("warnings")
	r := &warningsResponse{}
	err = u.CallJSON(r)
	return r.Warnings, err
}

func (s *Sabnzbd) Categories() (categories []string, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("get_cats")
	r := &categoriesResponse{}
	err = u.CallJSON(r)
	return r.Categories, err
}

func (s *Sabnzbd) Scripts() (scripts []string, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("get_scripts")
	r := &scriptsResponse{}
	err = u.CallJSON(r)
	return r.Scripts, err
}

func (s *Sabnzbd) Restart() (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("restart")
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) Delete(removeFiles bool, nzos ...string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("queue")
	u.v.Set("name", "delete")
	u.v.Set("value", strings.Join(nzos, ","))
	if removeFiles {
		u.v.Set("del_files", "1")
	}
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) DeleteAll(removeFiles bool) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("queue")
	u.v.Set("name", "delete")
	u.v.Set("value", "all")
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

// todo deal with return value { "result": { "priority": int, "position": int } }.
func (s *Sabnzbd) Move(nzo1, nzo2 string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("switch")
	u.v.Set("value", nzo1)
	u.v.Set("value2", nzo2)
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) MoveByPriority(nzo string, priority int) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("switch")
	u.v.Set("value", nzo)
	u.v.Set("value2", strconv.Itoa(priority))
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) Pause() (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("pause")
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) Resume() (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("resume")
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

// PauseTemporarily will pause for a time duration. The lowest possible value
// is one minute. Durations below one minute will resume the queue.
func (s *Sabnzbd) PauseTemporarily(t time.Duration) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("config")
	u.v.Set("name", "set_pause")
	u.v.Set("value", strconv.Itoa(int(t.Minutes())))
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) Shutdown() (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("shutdown")
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

type addNzbConfig struct {
	UnpackingOption *int
	Script          string
	Category        string
	XCategory       string
	Priority        *int
	NzbName         string
	Name            string
}

func (c *addNzbConfig) options() map[string]string {
	opts := map[string]string{}
	if c.UnpackingOption != nil {
		opts["pp"] = strconv.Itoa(*c.UnpackingOption)
	}
	if c.Script != "" {
		opts["script"] = c.Script
	}
	if c.Category != "" {
		opts["cat"] = c.Category
	}
	if c.XCategory != "" {
		opts["xcat"] = c.XCategory
	}
	if c.Priority != nil {
		opts["priority"] = strconv.Itoa(*c.Priority)
	}
	if c.NzbName != "" {
		opts["nzbname"] = c.NzbName
	}
	if c.Name != "" {
		opts["name"] = c.Name
	}
	return opts
}

type AddNzbOption func(*addNzbConfig) error

func AddNzbUnpackingOption(unpackingOption int) AddNzbOption {
	return func(c *addNzbConfig) error {
		c.UnpackingOption = &unpackingOption
		return nil
	}
}

func AddNzbScript(script string) AddNzbOption {
	return func(c *addNzbConfig) error {
		c.Script = script
		return nil
	}
}

func AddNzbCategory(category string) AddNzbOption {
	return func(c *addNzbConfig) error {
		c.Category = category
		return nil
	}
}

func AddNzbXCategory(xcategory string) AddNzbOption {
	return func(c *addNzbConfig) error {
		c.XCategory = xcategory
		return nil
	}
}

func AddNzbPriority(priority int) AddNzbOption {
	return func(c *addNzbConfig) error {
		c.Priority = &priority
		return nil
	}
}

func AddNzbName(name string) AddNzbOption {
	return func(c *addNzbConfig) error {
		c.NzbName = name
		return nil
	}
}

func AddNzbURL(urlv string) AddNzbOption {
	return func(c *addNzbConfig) error {
		c.Name = urlv
		return nil
	}
}

func (s *Sabnzbd) AddReader(reader io.Reader, filename string, options ...AddNzbOption) (nzoids []string, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("addfile")
	c := &addNzbConfig{}
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}
	for k, v := range c.options() {
		u.v.Set(k, v)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	pr, pw := io.Pipe()
	m := multipart.NewWriter(pw)
	contentType := m.FormDataContentType()
	go func() (err error) {
		defer wg.Done()
		defer func() {
			if err != nil {
				pw.CloseWithError(err)
			} else {
				pw.Close()
			}
		}()
		defer func() {
			mErr := m.Close()
			if err == nil {
				err = mErr
			}
		}()
		ffw, err := m.CreateFormFile("nzbfile", filename)
		if err != nil {
			return err
		}
		_, err = io.Copy(ffw, reader)
		return err
	}()

	r := &addFileResponse{}
	err = u.CallJSONMultipart(pr, contentType, r)
	wg.Wait()
	return r.NzoIDs, err
}

func (s *Sabnzbd) AddURL(options ...AddNzbOption) (nzoids []string, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("addurl")
	c := &addNzbConfig{}
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}
	for k, v := range c.options() {
		u.v.Set(k, v)
	}

	r := &addFileResponse{}
	err = u.CallJSON(r)
	return r.NzoIDs, err
}

func (s *Sabnzbd) AddFile(filename string, options ...AddNzbOption) (nzoids []string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.AddReader(f, filepath.Base(filename), options...)
}

func (s *Sabnzbd) AddLocalfile(filename string, options ...AddNzbOption) (nzoids []string, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("addlocalfile")
	u.v.Set("name", filename)
	c := &addNzbConfig{}
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}
	for k, v := range c.options() {
		u.v.Set(k, v)
	}
	r := &addFileResponse{}
	err = u.CallJSON(r)
	return r.NzoIDs, err
}

func (s *Sabnzbd) ChangeScript(nzoid, script string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("change_script")
	u.v.Set("value", nzoid)
	u.v.Set("value2", script)
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) ChangeCategory(nzoid, category string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("change_cat")
	u.v.Set("value", nzoid)
	u.v.Set("value2", category)
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

type QueueCompleteAction uint

const (
	QueueCompleteShutdownPC QueueCompleteAction = iota
	QueueCompleteHibernatePC
	QueueCompleteStandbyPC
	QueueCompleteShutdownProgram
	queueCompleteActions
)

var queueCompleteActionNames = []string{
	QueueCompleteShutdownPC:      "shutdown_pc",
	QueueCompleteHibernatePC:     "hibernate_pc",
	QueueCompleteStandbyPC:       "standby_pc",
	QueueCompleteShutdownProgram: "shutdown_program",
}

func (s *Sabnzbd) ChangeQueueCompleteAction(action QueueCompleteAction) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("change_complete_action")
	if action < queueCompleteActions {
		u.v.Set("value", queueCompleteActionNames[action])
	} else {
		return ErrInvalidQueueCompleteAction
	}
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

type PostProcessingMethod uint

const (
	PostProcessingSkip PostProcessingMethod = iota
	PostProcessingRepair
	PostProcessingRepairUnpack
	PostProcessingRepairUnpackDelete
)

func (s *Sabnzbd) ChangePostProcessing(nzoid string, method PostProcessingMethod) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("change_opts")
	u.v.Set("value", nzoid)
	u.v.Set("value2", fmt.Sprintf("%d", method))
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

type PriorityType int

const (
	PriorityDefault PriorityType = -100
	PriorityPaused  PriorityType = -2
	PriorityLow     PriorityType = -1
	PriorityNormal  PriorityType = 0
	PriorityHigh    PriorityType = 1
	PriorityForced  PriorityType = 2
)

func (s *Sabnzbd) ChangePriority(nzoid string, priority PriorityType) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("queue")
	u.v.Set("name", "priority")
	u.v.Set("value", nzoid)
	u.v.Set("value2", fmt.Sprintf("%d", priority))
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) PauseItem(nzoid string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("queue")
	u.v.Set("name", "pause")
	u.v.Set("value", nzoid)
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) ResumeItem(nzoid string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("queue")
	u.v.Set("name", "resume")
	u.v.Set("value", nzoid)
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) GetItemFiles(nzoid string) (files []ItemFile, err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("get_files")
	u.v.Set("value", nzoid)
	r := &ItemFilesResponse{}
	err = u.CallJSON(r)
	return r.Files, err
}

func (s *Sabnzbd) ChangeName(nzoid, name string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("queue")
	u.v.Set("name", "rename")
	u.v.Set("value", nzoid)
	u.v.Set("value2", name)
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) PausePostProcessing() (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("pause_pp")
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) ResumePostProcessing() (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("resume_pp")
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) DeleteHistory(removeFailedFiles bool, nzos ...string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("history")
	u.v.Set("name", "delete")
	u.v.Set("value", strings.Join(nzos, ","))
	if removeFailedFiles {
		u.v.Set("del_files", "1")
	}
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) DeleteAllHistory(removeFailedFiles bool) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("history")
	u.v.Set("name", "delete")
	u.v.Set("value", "all")
	if removeFailedFiles {
		u.v.Set("del_files", "1")
	}
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) DeleteFailedHistory(removeFailedFiles bool) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("history")
	u.v.Set("name", "delete")
	u.v.Set("value", "failed")
	if removeFailedFiles {
		u.v.Set("del_files", "1")
	}
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) Retry(nzoid string) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("retry")
	u.v.Set("value", nzoid)
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}

func (s *Sabnzbd) SpeedLimit(kbps int) (err error) {
	u := s.url()
	u.SetJSONOutput()
	u.Authenticate()
	u.SetMode("config")
	u.v.Set("name", "speedlimit")
	u.v.Set("value", strconv.Itoa(kbps))
	r := &apiError{}
	err = u.CallJSON(r)
	return err
}
