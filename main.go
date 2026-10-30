// qrwifi shares the current Wi-Fi network as a QR code and joins a network
// from a Wi-Fi QR code (image file or raw WIFI: string). Requires nmcli.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	qrcodegen "github.com/skip2/go-qrcode"
)

const usage = `usage:
  qrwifi show [-s SSID] [-o file.png]   show QR for the active (or named) saved network
  qrwifi connect <image.png|"WIFI:..."> join the network encoded in a QR image or string
`

type network struct {
	SSID, Password, Auth string // Auth: WPA, WEP or nopass
	Hidden               bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "show":
		err = cmdShow(os.Args[2:])
	case "connect":
		err = cmdConnect(os.Args[2:])
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "qrwifi:", err)
		os.Exit(1)
	}
}

func nmcli(args ...string) (string, error) {
	var out, stderr bytes.Buffer
	cmd := exec.Command("nmcli", args...)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("nmcli %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// escape applies the WIFI: payload escaping for \ ; , : and ".
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, `:`, `\:`, `"`, `\"`).Replace(s)
}

func (n network) payload() string {
	s := "WIFI:T:" + n.Auth + ";S:" + escape(n.SSID) + ";"
	if n.Auth != "nopass" {
		s += "P:" + escape(n.Password) + ";"
	}
	if n.Hidden {
		s += "H:true;"
	}
	return s + ";"
}

// parsePayload parses a WIFI:T:...;S:...;P:...;; string.
func parsePayload(s string) (network, error) {
	var n network
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(strings.ToUpper(s), "WIFI:") {
		return n, errors.New("not a WIFI: payload")
	}
	s = s[len("WIFI:"):]
	var key, val strings.Builder
	inVal := false
	flush := func() {
		v := val.String()
		switch strings.ToUpper(key.String()) {
		case "T":
			n.Auth = v
		case "S":
			n.SSID = v
		case "P":
			n.Password = v
		case "H":
			n.Hidden = strings.EqualFold(v, "true")
		}
		key.Reset()
		val.Reset()
		inVal = false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			i++
			val.WriteByte(s[i])
		case c == ':' && !inVal:
			inVal = true
		case c == ';':
			flush()
		case inVal:
			val.WriteByte(c)
		default:
			key.WriteByte(c)
		}
	}
	if n.SSID == "" {
		return n, errors.New("payload has no SSID")
	}
	return n, nil
}

func activeSSID() (string, error) {
	out, err := nmcli("-t", "-f", "ACTIVE,SSID", "device", "wifi")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(line, "yes:"); ok {
			return strings.ReplaceAll(rest, `\:`, ":"), nil
		}
	}
	return "", errors.New("not connected to a Wi-Fi network")
}

func lookup(ssid string) (network, error) {
	n := network{SSID: ssid}
	// Connection profile names usually equal the SSID.
	get := func(field string) string {
		v, _ := nmcli("-s", "-g", field, "connection", "show", ssid)
		return v
	}
	if get("802-11-wireless.ssid") == "" {
		return n, fmt.Errorf("no saved connection %q (profile name must match the SSID)", ssid)
	}
	n.Hidden = get("802-11-wireless.hidden") == "yes"
	switch get("802-11-wireless-security.key-mgmt") {
	case "":
		n.Auth = "nopass"
	case "none":
		n.Auth, n.Password = "WEP", get("802-11-wireless-security.wep-key0")
	default:
		n.Auth, n.Password = "WPA", get("802-11-wireless-security.psk")
	}
	return n, nil
}

func cmdShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	ssid := fs.String("s", "", "SSID (default: active network)")
	out := fs.String("o", "", "write PNG to this file instead of the terminal")
	fs.Parse(args)

	if *ssid == "" {
		s, err := activeSSID()
		if err != nil {
			return err
		}
		*ssid = s
	}
	n, err := lookup(*ssid)
	if err != nil {
		return err
	}
	qr, err := qrcodegen.New(n.payload(), qrcodegen.Medium)
	if err != nil {
		return err
	}
	if *out != "" {
		return qr.WriteFile(512, *out)
	}
	fmt.Printf("%s (%s)\n", n.SSID, n.Auth)
	printTerminal(qr.Bitmap())
	return nil
}

// printTerminal renders the bitmap two rows per line using half blocks.
func printTerminal(bm [][]bool) {
	at := func(y, x int) bool { return y < len(bm) && bm[y][x] }
	for y := 0; y < len(bm); y += 2 {
		for x := range bm[y] {
			switch top, bot := at(y, x), at(y+1, x); {
			case top && bot:
				fmt.Print("█")
			case top:
				fmt.Print("▀")
			case bot:
				fmt.Print("▄")
			default:
				fmt.Print(" ")
			}
		}
		fmt.Println()
	}
}

func decodeImage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		return "", fmt.Errorf("no QR code found in %s: %v", path, err)
	}
	return res.GetText(), nil
}

func cmdConnect(args []string) error {
	if len(args) != 1 {
		return errors.New(`connect needs one argument: an image path or a "WIFI:..." string`)
	}
	text := args[0]
	if !strings.HasPrefix(strings.ToUpper(text), "WIFI:") {
		var err error
		if text, err = decodeImage(text); err != nil {
			return err
		}
	}
	n, err := parsePayload(text)
	if err != nil {
		return err
	}
	cmd := []string{"device", "wifi", "connect", n.SSID}
	if n.Auth != "nopass" && n.Password != "" {
		cmd = append(cmd, "password", n.Password)
	}
	if n.Hidden {
		cmd = append(cmd, "hidden", "yes")
	}
	out, err := nmcli(cmd...)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}
