package cpe

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBillConsultSOAPURL = "https://e-factura.sunat.gob.pe/ol-it-wsconscpegem/billConsultService"

// ConsultarCdrSOAP invoca el Web Service SOAP oficial billConsultService (getStatusCdr) de SUNAT
// usando las credenciales SOL de la empresa para descargar la Constancia de Recepción (CDR .zip).
func ConsultarCdrSOAP(endpoint, rucEmpresa, usuarioSOL, claveSOL, rucEmisor, tipoCP, serie, numero string) ([]byte, string, string, error) {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultBillConsultSOAPURL
	}

	numClean := strings.TrimSpace(numero)
	numInt, err := strconv.Atoi(strings.TrimLeft(numClean, "0"))
	if err != nil || numInt <= 0 {
		// Si es "0" o alfanumérico, probar directo
		if n2, e2 := strconv.Atoi(numClean); e2 == nil {
			numInt = n2
		} else {
			return nil, "", "", fmt.Errorf("el número de comprobante '%s' debe ser numérico para consultar CDR en SUNAT", numero)
		}
	}

	rucEmisor = strings.TrimSpace(rucEmisor)
	if rucEmisor == "" {
		rucEmisor = strings.TrimSpace(rucEmpresa)
	}

	username := escapeXML(strings.TrimSpace(rucEmpresa) + strings.TrimSpace(usuarioSOL))
	password := escapeXML(strings.TrimSpace(claveSOL))

	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
                  xmlns:ser="http://service.sunat.gob.pe"
                  xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
  <soapenv:Header>
    <wsse:Security>
      <wsse:UsernameToken>
        <wsse:Username>%s</wsse:Username>
        <wsse:Password>%s</wsse:Password>
      </wsse:UsernameToken>
    </wsse:Security>
  </soapenv:Header>
  <soapenv:Body>
    <ser:getStatusCdr>
      <rucComprobante>%s</rucComprobante>
      <tipoComprobante>%s</tipoComprobante>
      <serieComprobante>%s</serieComprobante>
      <numeroComprobante>%d</numeroComprobante>
    </ser:getStatusCdr>
  </soapenv:Body>
</soapenv:Envelope>`,
		username,
		password,
		escapeXML(rucEmisor),
		escapeXML(padTipoCP(tipoCP)),
		escapeXML(strings.ToUpper(strings.TrimSpace(serie))),
		numInt,
	)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(envelope))
	if err != nil {
		return nil, "", "", fmt.Errorf("error preparando petición SOAP SUNAT: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "urn:getStatusCdr")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("error de red al consultar billConsultService de SUNAT: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("error leyendo respuesta SOAP de SUNAT: %w", err)
	}

	statusCode, statusMsg, contentB64, faultCode, faultString := parseSOAPStatusCdrResponse(bodyBytes)
	if faultCode != "" || faultString != "" {
		return nil, faultCode, faultString, fmt.Errorf("SUNAT SOAP rechazó la consulta (%s): %s", faultCode, faultString)
	}

	if resp.StatusCode != http.StatusOK && statusCode == "" {
		snippet := strings.TrimSpace(string(bodyBytes))
		if len(snippet) > 250 {
			snippet = snippet[:250] + "..."
		}
		return nil, "", "", fmt.Errorf("SUNAT SOAP respondió HTTP %d: %s", resp.StatusCode, snippet)
	}

	var cdrBytes []byte
	if contentB64 != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimSpace(contentB64), "\n", ""))
		if decErr != nil {
			return nil, statusCode, statusMsg, fmt.Errorf("error decodificando CDR Base64 de SUNAT: %w", decErr)
		}
		cdrBytes = decoded
	}

	return cdrBytes, statusCode, statusMsg, nil
}

func parseSOAPStatusCdrResponse(data []byte) (statusCode, statusMsg, contentB64, faultCode, faultString string) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var current string

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			current = t.Name.Local
		case xml.EndElement:
			current = ""
		case xml.CharData:
			val := strings.TrimSpace(string(t))
			if val == "" || current == "" {
				continue
			}
			switch current {
			case "statusCode":
				statusCode = val
			case "statusMessage":
				statusMsg = val
			case "content":
				contentB64 += val
			case "faultcode":
				faultCode = val
			case "faultstring":
				faultString = val
			}
		}
	}
	return
}

func escapeXML(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
