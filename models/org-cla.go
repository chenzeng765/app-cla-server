package models

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/opensourceways/app-cla-server/dbmodels"
)

type OrgRepoCreateOption struct {
	Platform string `json:"platform"`
	OrgID    string `json:"org_id"`
	RepoID   string `json:"repo_id"`

	ApplyTo string `json:"apply_to"`

	OrgEmail string `json:"org_email"`

	CLA CLACreateOption `json:"cla"`
}

func (this OrgRepoCreateOption) Create(claID string) (string, error) {
	info := dbmodels.CLAOrg{
		Platform:    this.Platform,
		OrgID:       this.OrgID,
		RepoID:      this.RepoID,
		ApplyTo:     this.ApplyTo,
		OrgEmail:    this.OrgEmail,
		Enabled:     true,
		CLAID:       claID,
		CLALanguage: this.CLA.Language,
	}
	return dbmodels.GetDB().CreateBindingBetweenCLAAndOrg(info)
}

type CLACreateOption struct {
	content  []byte
	URL      string           `json:"url"`
	Language string           `json:"language"`
	Fields   []dbmodels.Field `json:"fields"`
}

func (this *CLACreateOption) Create() (string, error) {
	cla := dbmodels.CLA{
		Name:     this.URL,
		Text:     string(this.content),
		Language: this.Language,
		Fields:   this.Fields,
	}
	return dbmodels.GetDB().CreateCLA(cla)
}

func (this *CLACreateOption) DownloadCLA() error {
	var resp *http.Response

	for i := 0; i < 3; i++ {
		v, err := http.Get(this.URL)
		if err == nil {
			resp = v
			break
		}
		time.Sleep(time.Second * time.Duration(1))
	}
	if resp == nil {
		return fmt.Errorf("can't download %s", this.URL)
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if strings.HasPrefix(http.DetectContentType(data), "text/plain") {
		this.content = data
		return nil
	}

	return fmt.Errorf("it is not the content of cla")
}