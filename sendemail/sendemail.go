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
import "gopkg.in/hlandau/easyconfig.v1/cflag"
import "path/filepath"
import "github.com/hlandau/xlog"
import "sync"
import "io"
import "gopkg.in/alexcesaro/quotedprintable.v3"
import "mime/multipart"
import "net/textproto"

var cEmailsSent = cexp.NewCounter("sendemail.emailsSent")

var log, Log = xlog.New("sendemail")

var (
	fg               = cflag.NewGroup(nil, "sendemail")
	smtpAddressFlag  = cflag.String(fg, "smtpaddress", "", "SMTP address (hostname[:port])")
	smtpUsernameFlag = cflag.String(fg, "smtpusername", "", "SMTP username")
	smtpPasswordFlag = cflag.String(fg, "smtppassword", "", "SMTP password")
	sendmailPathFlag = cflag.String(fg, "sendmailpath", "", "path to /usr/sbin/sendmail")
	numSendersFlag   = cflag.Int(fg, "numsenders", 2, "number of asynchronous e. mail senders")
	fromFlag         = cflag.String(fg, "from", "nobody@localhost", "Default from address")
)

var sendChan = make(chan *Email, 32)
var startOnce sync.Once

func sendLoop() {
	for e := range sendChan {
		err := Send(e)
		log.Errore(err, "cannot send e. mail")
	}
}

func startSenders() {
	startOnce.Do(func() {
		numSenders := numSendersFlag.Value()
		for i := 0; i < numSenders; i++ {
			go sendLoop()
		}
	})
}

type Email struct {
	From             string
	To               []string
	Headers          mail.Header
	Body             string
	OpenPGPEncryptTo []string

	// If Body is "", a message is assembled as follows:
	//
	//                   TextBody not set  TextBody set
	// HTMLBody not set  Empty message     Non-MIME text message
	// HTMLBody set      MIME HTML-only    MIME HTML & text message
	//
	HTMLBody string
	TextBody string

	rfc822Message []byte
}

func SendAsync(e *Email) {
	startSenders()
	sendChan <- e
}

func Send(e *Email) error {
	if len(e.From) == 0 {
		v := fromFlag.Value()
		if _, err := mail.ParseAddress(v); err != nil {
			return fmt.Errorf("no from address specified and default from address is not valid")
		}

		e.From = v
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

	err := e.assembleMIME()
	if err != nil {
		return err
	}

	err = encryptEmail(e)
	if err != nil {
		return err
	}

	e.rfc822Message = nil
	e.rfc822Message = append(e.rfc822Message, serializeHeaders(e.Headers)...)
	e.rfc822Message = append(e.rfc822Message, '\n')
	e.rfc822Message = append(e.rfc822Message, e.Body...)

	cEmailsSent.Add(1)
	return send(e)
}

func send(e *Email) error {
	if smtpAddressFlag.Value() != "" {
		return sendViaSMTP(e)
	}

	autodetectSendmail()

	return sendViaSendmail(e)
}

var triedAutodetection = false

var guessPaths = []string{
	"/usr/sbin/sendmail",
}

func autodetectSendmail() {
	if triedAutodetection || sendmailPathFlag.Value() != "" {
		return
	}

	triedAutodetection = true
	p, err := exec.LookPath("sendmail")
	if err == nil {
		p2, err := filepath.Abs(p)
		if err == nil {
			p = p2
		}

		sendmailPathFlag.SetValue(p)
		return
	}

	for _, p := range guessPaths {
		_, err := os.Stat(p)
		if err != nil {
			continue
		}

		sendmailPathFlag.SetValue(p)
		return
	}
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

func sendViaSMTP(e *Email) error {
	var auth smtp.Auth

	if smtpUsernameFlag.Value() != "" {
		host, _, err := net.SplitHostPort(smtpAddressFlag.Value())
		if err != nil {
			return err
		}

		auth = smtp.PlainAuth("", smtpUsernameFlag.Value(), smtpPasswordFlag.Value(), host)
	}

	return smtp.SendMail(smtpAddressFlag.Value(), auth, e.From, e.To, e.rfc822Message)
}

func sendViaSendmail(e *Email) error {
	smargs := []string{"-i"}
	smargs = append(smargs, e.To...)

	spath := sendmailPathFlag.Value()
	if spath == "" {
		return fmt.Errorf("neither SMTP nor sendmail configured, and sendmail cannot be detected")
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

func (e *Email) assembleMIME() error {
	if e.Body != "" || (e.TextBody == "" && e.HTMLBody == "") {
		return nil
	}

	if e.TextBody != "" && e.HTMLBody == "" {
		e.Body = e.TextBody
		return nil
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	appendPart(w, func(h textproto.MIMEHeader) {
		h.Set("Content-Type", "text/plain; charset=utf-8")
	}, e.TextBody)

	appendPart(w, func(h textproto.MIMEHeader) {
		h.Set("Content-Type", "text/html; charset=utf-8")
	}, e.HTMLBody)

	e.Headers["MIME-Version"] = []string{"1.0"}
	e.Headers["Content-Type"] = []string{"multipart/alternative; boundary=" + w.Boundary()}
	e.Body = string(buf.Bytes())
	return nil
}

func appendPart(w *multipart.Writer, headers func(h textproto.MIMEHeader), body string) {
	if body == "" {
		return
	}

	h := textproto.MIMEHeader{}
	h.Set("Content-Transfer-Encoding", "quoted-printable")
	headers(h)
	partW, err := w.CreatePart(h)
	log.Panice(err, "create MIME part")
	quoW := quotedprintable.NewWriter(partW)
	defer quoW.Close()
	io.WriteString(quoW, body)
}
