package msiso

import (
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/google/uuid"
)

type WindowsLang string

type WindowsDownload interface {
	GetLanguages() ([]WindowsLang, error)
	GetISOLink(lang WindowsLang) (string, error)
}

type WindowsEdition int

const (
	Win10 WindowsEdition = iota
	Win11
	// kWin10EnterpriseLTSC //TODO
)

type msdlState struct {
	Edition      WindowsEdition
	SKUs         map[string]string
	StartUrl     string
	ProductShort string
	SessionID    string
}

var productIDMatcher = regexp.MustCompile(`<option\svalue="([0-9]+)">Windows`)
var langIDMatcher = regexp.MustCompile(`<option\svalue=".*;([0-9]+)&.*;([a-zA-Z\(\)\s]+)&.*">`)
var iso64matcher = regexp.MustCompile(`href="(https:\/\/.*)">.*>IsoX64`)

func (s *msdlState) GetLanguages() ([]WindowsLang, error) {

	req, _ := http.NewRequest(http.MethodGet, s.StartUrl, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:100.0) Gecko/20100101 Firefox/100.0") // simulate linux to get direct links instead of mediacreationtool.exe

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var matches = productIDMatcher.FindStringSubmatch(string(body))

	if len(matches) != 2 {
		return nil, fmt.Errorf("regex didnt match product ids")
	}

	var productID = matches[1]

	resp, err = http.Get("https://www.microsoft.com/en-US/api/controls/contentinclude/html?pageId=a8f8f489-4c7f-463a-9ca6-5cff94d8d041&host=www.microsoft.com&segments=software-download," + s.ProductShort + "&query=&action=getskuinformationbyproductedition&sessionId=" + s.SessionID + "&productEditionId=" + productID + "&sdVersion=2")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)

	var langMatches = langIDMatcher.FindAllStringSubmatch(string(body), -1)

	var langs = make([]WindowsLang, 0)
	for _, v := range langMatches {
		s.SKUs[v[2]] = v[1]
		langs = append(langs, WindowsLang(v[2]))
	}

	return langs, nil

}

func (s *msdlState) GetISOLink(lang WindowsLang) (string, error) {

	if s.SKUs[string(lang)] == "" {
		return "", fmt.Errorf("SKU '%s' not found", lang)
	}

	req, _ := http.NewRequest(
		http.MethodGet,
		"https://www.microsoft.com/en-US/api/controls/contentinclude/html?pageId=6e2a1789-ef16-4f27-a296-74ef7ef5d96b&host=www.microsoft.com&segments=software-download,"+s.ProductShort+"&query=&action=GetProductDownloadLinksBySku&sessionId="+s.SessionID+"&skuId="+s.SKUs[string(lang)]+"&language="+string(lang)+"&sdVersion=2",
		nil,
	)

	req.Header.Add("Referer", s.StartUrl)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var isoLinkMatches = iso64matcher.FindStringSubmatch(string(body))

	if len(isoLinkMatches) != 2 {
		return "", fmt.Errorf("regex didnt match any isos")
	}

	return isoLinkMatches[1], nil

}

func GetDownload(edition WindowsEdition) WindowsDownload {

	var productUrl string

	var createSession = func() string {
		v4, _ := uuid.NewRandom()

		resp, _ := http.Get("https://vlscppe.microsoft.com/tags?org_id=y6jn8c31&session_id=" + v4.String())
		resp.Body.Close()
		return v4.String()
	}

	var session string

	switch edition {
	case Win10:
		productUrl = "windows10ISO"
		session = createSession()
	case Win11:
		productUrl = "windows11"
		session = createSession()
	default:
	}

	var baseurl = "https://www.microsoft.com/en-us/software-download/" + productUrl

	return &msdlState{
		Edition:      edition,
		SKUs:         make(map[string]string),
		StartUrl:     baseurl,
		ProductShort: productUrl,
		SessionID:    session,
	}

}
