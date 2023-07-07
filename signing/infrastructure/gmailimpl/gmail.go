package gmailimpl

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"text/template"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	gooleoauth2 "google.golang.org/api/oauth2/v2"
	option "google.golang.org/api/option"

	myoauth2 "github.com/opensourceways/app-cla-server/oauth2"
	"github.com/opensourceways/app-cla-server/util"
)

type gmailClient struct {
	cfg       *oauth2.Config
	emailTemp *template.Template
}

func (this *gmailClient) initialize(credentials []byte) error {
	cfg, err := this.getOauth2Config(credentials)
	if err != nil {
		return fmt.Errorf("Failtd to initialize gmail client: %s", err.Error())
	}

	str := emailTempWithAttachmentForGmail()
	tmpl, err := template.New("email").Parse(str)
	if err != nil {
		return fmt.Errorf("Failtd to initialize gmail client: %s", err.Error())
	}

	this.emailTemp = tmpl
	this.cfg = cfg
	return nil
}

func (this *gmailClient) GetToken(code, scope string) (*oauth2.Token, error) {
	if this.cfg == nil {
		return nil, fmt.Errorf("gmail has not been initialized")
	}

	return myoauth2.FetchOauth2Token(this.cfg, code)
}

func (this *gmailClient) GetAuthorizedEmail(token *oauth2.Token) (string, error) {
	ctx := context.Background()
	srv, err := gooleoauth2.NewService(
		ctx, option.WithTokenSource(this.cfg.TokenSource(ctx, token)))
	if err != nil {
		return "", err
	}

	info, err := srv.Userinfo.V2.Me.Get().Do()
	if err != nil {
		return "", err
	}
	return info.Email, nil
}

func (this *gmailClient) GetOauth2CodeURL(state string) string {
	return myoauth2.GetOauth2CodeURL(this.cfg, state)
}

func (this *gmailClient) sendEmail(token *oauth2.Token, msg *EmailMessage) error {
	client := this.cfg.Client(context.Background(), token)
	srv, err := gmail.New(client)
	if err != nil {
		return err
	}

	msg1, err := this.createGmailMessage(msg)
	if err != nil {
		return err
	}

	_, err = srv.Users.Messages.Send("me", msg1).Do()

	return err
}

func (this *gmailClient) getOauth2Config(credentials []byte) (*oauth2.Config, error) {
	config, err := google.ConfigFromJSON(credentials, this.getScope()...)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client secret file to config: %v", err)
	}

	return config, nil
}

func (this *gmailClient) getScope() []string {
	return []string{gooleoauth2.UserinfoEmailScope, gmail.GmailSendScope}
}

func (this *gmailClient) createGmailMessage(msg *EmailMessage) (*gmail.Message, error) {
	attachment := msg.Attachment
	if attachment == "" {
		return simpleGmailMessage(msg), nil
	}

	fileBytes, err := ioutil.ReadFile(attachment)
	if err != nil {
		return nil, fmt.Errorf("Unable to read file for attachment: %s", err.Error())
	}

	data := struct {
		To           string
		Subject      string
		Content      string
		Boundary     string
		FileName     string
		FileData     string
		FileMIMEType string
	}{
		To:           msg.To[0],
		Subject:      msg.Subject,
		Content:      msg.Content,
		Boundary:     util.RandStr(32, "alphanum"),
		FileData:     base64.StdEncoding.EncodeToString(fileBytes),
		FileName:     path.Base(attachment),
		FileMIMEType: http.DetectContentType(fileBytes),
	}

	buf := new(bytes.Buffer)
	err = this.emailTemp.Execute(buf, data)
	if err != nil {
		return nil, fmt.Errorf("parse template failed: %s", err.Error())
	}

	return &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

func simpleGmailMessage(msg *EmailMessage) *gmail.Message {
	to := strings.Join(msg.To, "; ")
	raw := fmt.Sprintf("To: %s\r\nSubject: %s\r\n%s\r\n%s", to, msg.Subject, msg.MIME, msg.Content)

	return &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(raw)),
	}
}

func emailTempWithAttachmentForGmail() string {
	return `Content-Type: multipart/mixed; boundary={{.Boundary}}
MIME-Version: 1.0
to: {{.To}}
subject: {{.Subject}}

--{{.Boundary}}
Content-Type: text/plain; charset="UTF-8"
MIME-Version: 1.0
Content-Transfer-Encoding: 7bit

{{.Content}}

--{{.Boundary}}
Content-Type: {{.FileMIMEType}}; name="{{.FileName}}"
MIME-Version: 1.0
Content-Transfer-Encoding: base64
Content-Disposition: attachment; filename="{{.FileName}}"

{{.FileData}}

--{{.Boundary}}--`
}