package datasets

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Service) Export(ctx context.Context, id string, format string) ([]byte, string, string, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return nil, "", "", "invalid_id", nil
	}

	exportFormat := normalizeExportFormat(format)
	if exportFormat == "" {
		return nil, "", "", "unsupported_export_format", nil
	}

	var d DatasetDoc
	err = datasetFindOneFn(ctx, s.db, "datasets", bson.M{"_id": oid}).Decode(&d)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, "", "", "not_found", nil
		}
		return nil, "", "", "", err
	}

	ballots, err := s.datasetBallots(ctx, oid)
	if err != nil {
		return nil, "", "", "", err
	}
	if len(ballots) == 0 {
		return nil, "", "", "no_ballots", nil
	}

	rows := datasetExportRows(d, ballots)
	base := safeDatasetFilenameBase(d.Name)

	switch exportFormat {
	case "csv":
		data, err := writeDelimitedRows(rows, ',')
		if err != nil {
			return nil, "", "", "", err
		}
		return data, base + ".csv", "text/csv; charset=utf-8", "", nil

	case "txt":
		data, err := writeDelimitedRows(rows, '\t')
		if err != nil {
			return nil, "", "", "", err
		}
		return data, base + ".txt", "text/plain; charset=utf-8", "", nil

	case "xlsx":
		data, err := writeXLSXRows(rows)
		if err != nil {
			return nil, "", "", "", err
		}
		return data, base + ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "", nil

	default:
		return nil, "", "", "unsupported_export_format", nil
	}
}

func normalizeExportFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "csv":
		return "csv"
	case "txt":
		return "txt"
	case "xlsx", "xls", "xlx":
		return "xlsx"
	default:
		return ""
	}
}

func (s *Service) datasetBallots(ctx context.Context, datasetID primitive.ObjectID) ([]BallotDoc, error) {
	cur, err := datasetFindFn(
		ctx,
		s.db,
		"dataset_ballots",
		bson.M{"dataset_id": datasetID},
		options.Find().SetSort(bson.D{{Key: "voter_ref", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []BallotDoc
	for cur.Next(ctx) {
		var b BallotDoc
		if err := cur.Decode(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func datasetExportRows(d DatasetDoc, ballots []BallotDoc) [][]string {
	rows := [][]string{
		{"record_type", "id", "name", "voter_ref", "approval", "ranking", "scores"},
	}

	for _, c := range d.Candidates {
		rows = append(rows, []string{
			"candidate",
			strings.TrimSpace(c.ID),
			strings.TrimSpace(c.Name),
			"",
			"",
			"",
			"",
		})
	}

	for idx, b := range ballots {
		voterRef := strings.TrimSpace(b.VoterRef)
		if voterRef == "" {
			voterRef = "v" + itoa(idx+1)
		}

		row := []string{"ballot", "", "", voterRef, "", "", ""}

		switch d.Format {
		case "approval":
			row[4] = strings.Join(b.Approval, "|")
		case "ranking":
			row[5] = strings.Join(b.Ranking, "|")
		case "score":
			row[6] = scoresExportCell(d.Candidates, b.Scores)
		}

		rows = append(rows, row)
	}

	return rows
}

func scoresExportCell(candidates []Candidate, scores map[string]int) string {
	if len(scores) == 0 {
		return ""
	}

	used := map[string]struct{}{}
	parts := make([]string, 0, len(scores))

	for _, c := range candidates {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			continue
		}
		if score, ok := scores[id]; ok {
			parts = append(parts, id+"="+strconv.Itoa(score))
			used[id] = struct{}{}
		}
	}

	extraIDs := make([]string, 0)
	for id := range scores {
		if _, ok := used[id]; !ok {
			extraIDs = append(extraIDs, id)
		}
	}
	sort.Strings(extraIDs)

	for _, id := range extraIDs {
		parts = append(parts, id+"="+strconv.Itoa(scores[id]))
	}

	return strings.Join(parts, "|")
}

func writeDelimitedRows(rows [][]string, delimiter rune) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = delimiter

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func writeXLSXRows(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets>
<sheet name="dataset" sheetId="1" r:id="rId1"/>
</sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"xl/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
<fills count="1"><fill><patternFill patternType="none"/></fill></fills>
<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs>
</styleSheet>`,
		"xl/worksheets/sheet1.xml": buildSheetXML(rows),
	}

	fileNames := make([]string, 0, len(files))
	for name := range files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	for _, name := range fileNames {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func buildSheetXML(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	b.WriteString(`<sheetData>`)

	for rowIdx, row := range rows {
		excelRow := rowIdx + 1
		b.WriteString(`<row r="`)
		b.WriteString(strconv.Itoa(excelRow))
		b.WriteString(`">`)

		for colIdx, value := range row {
			cellRef := xlsxCellRef(colIdx+1, excelRow)
			b.WriteString(`<c r="`)
			b.WriteString(cellRef)
			b.WriteString(`" t="inlineStr"><is><t>`)
			b.WriteString(escapeXML(value))
			b.WriteString(`</t></is></c>`)
		}

		b.WriteString(`</row>`)
	}

	b.WriteString(`</sheetData>`)
	b.WriteString(`</worksheet>`)
	return b.String()
}

func xlsxCellRef(col, row int) string {
	return xlsxColumnName(col) + strconv.Itoa(row)
}

func xlsxColumnName(col int) string {
	if col <= 0 {
		return "A"
	}

	var out []byte
	for col > 0 {
		col--
		out = append([]byte{byte('A' + col%26)}, out...)
		col /= 26
	}

	return string(out)
}

func escapeXML(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

func safeDatasetFilenameBase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "dataset"
	}

	var b strings.Builder
	prevDash := false

	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_':
			b.WriteRune(r)
			prevDash = false
		case unicode.IsSpace(r) || r == '.' || r == '/':
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "dataset"
	}

	if len([]rune(out)) > 80 {
		runes := []rune(out)
		out = string(runes[:80])
		out = strings.Trim(out, "-_")
		if out == "" {
			return "dataset"
		}
	}

	return out
}

var _ = fmt.Sprintf