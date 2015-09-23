package sendemail

import "golang.org/x/crypto/openpgp"
import "golang.org/x/crypto/openpgp/armor"
import "golang.org/x/crypto/openpgp/packet"
import "fmt"
import "bytes"
import "net/mail"
import "net/smtp"
import "os"
import "os/exec"
import "net"
import "gopkg.in/hlandau/easymetric.v1/cexp"

var cEmailsSent = cexp.NewCounter("sendemail.emailsSent")

type Config struct {
	SMTPAddress  string
	SMTPUsername string
	SMTPPassword string

	SendmailPath string
}

type Email struct {
	From             string
	To               []string
	Headers          mail.Header
	Body             string
	OpenPGPEncryptTo []string

	rfc822Message []byte
}

type Sender interface {
	SendEmail(e *Email) error
}

func (cfg *Config) SendEmail(e *Email) error {
	if len(e.From) == 0 {
		return fmt.Errorf("from address must be specified")
	}

	if len(e.To) == 0 {
		return fmt.Errorf("at least one recipient must be specified")
	}

	if e.Headers == nil {
		e.Headers = make(map[string][]string)
	}

	if _, ok := e.Headers["From"]; !ok {
		e.Headers["From"] = []string{e.From}
	}

	if _, ok := e.Headers["To"]; !ok {
		e.Headers["To"] = e.To
	}

	err := encryptEmail(e)
	if err != nil {
		return err
	}

	e.rfc822Message = nil
	e.rfc822Message = append(e.rfc822Message, serializeHeaders(e.Headers)...)
	e.rfc822Message = append(e.rfc822Message, '\n')
	e.rfc822Message = append(e.rfc822Message, e.Body...)

	cEmailsSent.Add(1)

	if cfg.SMTPAddress != "" {
		return cfg.sendViaSMTP(e)
	}

	return cfg.sendViaSendmail(e)
}

func serializeHeaders(headers mail.Header) (b []byte) {
	for k, v := range headers {
		// XXX
		for _, x := range v {
			b = append(b, []byte(fmt.Sprintf("%s: %s\n", k, x))...)
		}
	}
	return
}

func encryptEmail(e *Email) error {
	if len(e.OpenPGPEncryptTo) == 0 {
		return nil
	}

	buf := &bytes.Buffer{}
	var destEntities []*openpgp.Entity
	for _, eto := range e.OpenPGPEncryptTo {
		r := bytes.NewBufferString(eto)
		blk, err := armor.Decode(r)
		if err != nil {
			return err
		}

		rr := packet.NewReader(blk.Body)
		e, err := openpgp.ReadEntity(rr)
		if err != nil {
			return err
		}

		destEntities = append(destEntities, e)
	}

	aew, err := armor.Encode(buf, "PGP MESSAGE", map[string]string{
		"Version": "OpenPGP",
	})
	if err != nil {
		return err
	}

	wr, err := openpgp.Encrypt(aew, destEntities, nil, nil, nil)
	if err != nil {
		return err
	}

	_, err = wr.Write([]byte(e.Body))
	if err != nil {
		wr.Close()
		return err
	}

	wr.Close()
	aew.Close()

	e.Body = string(buf.Bytes())
	return nil
}

func (cfg *Config) sendViaSMTP(e *Email) error {
	var auth smtp.Auth

	if cfg.SMTPUsername != "" {
		host, _, err := net.SplitHostPort(cfg.SMTPAddress)
		if err != nil {
			return err
		}

		auth = smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, host)
	}

	return smtp.SendMail(cfg.SMTPAddress, auth, e.From, e.To, e.rfc822Message)
}

func (cfg *Config) sendViaSendmail(e *Email) error {
	smargs := []string{"-i"}
	smargs = append(smargs, e.To...)

	spath := cfg.SendmailPath
	if spath == "" {
		spath = "/usr/sbin/sendmail"
	}

	sm := exec.Command(spath, smargs...)
	stdin, err := sm.StdinPipe()
	if err != nil {
		return err
	}

	sm.Stdout = os.Stdout
	sm.Stderr = os.Stderr
	err = sm.Start()
	if err != nil {
		return err
	}

	stdin.Write(e.rfc822Message)
	stdin.Close()

	err = sm.Wait()
	if err != nil {
		return err
	}

	return nil
}
