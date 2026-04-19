package datasets

import (
	"context"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *Service) Import(ctx context.Context, meta ImportMeta, fileHeader *multipart.FileHeader, file multipart.File) (string, string, error) {
	name := strings.TrimSpace(meta.Name)
	format := normalizeFormat(meta.Format)

	b, err := io.ReadAll(file)
	if err != nil {
		return "", "", err
	}

	mime := fileHeader.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	var parsed importFile
	parsedOK := false
	if p, ok := parseImportFile(b, fileHeader.Filename, mime); ok {
		parsed = p
		parsedOK = true
		if name == "" {
			name = strings.TrimSpace(parsed.Dataset.Name)
		}
		if format == "" {
			format = normalizeFormat(parsed.Dataset.Format)
		}
	}

	if name == "" {
		return "", "invalid_name", nil
	}
	if !isValidFormat(format) {
		return "", "invalid_format", nil
	}

	candidates := parsed.Dataset.Candidates
	params := parsed.Dataset.Parameters
	seed := parsed.Dataset.Seed
	desc := strings.TrimSpace(meta.Description)
	if desc == "" {
		desc = strings.TrimSpace(parsed.Dataset.Description)
	}

	if parsedOK {
		if len(parsed.Ballots) > 0 && len(candidates) == 0 {
			return "", "invalid_candidates", nil
		}
		if len(candidates) > 0 {
			if code := validateCandidates(candidates); code != "" {
				return "", code, nil
			}
			if code := validateDatasetParams(format, params, len(candidates), false); code != "" {
				return "", code, nil
			}
		}
	}

	doc := DatasetDoc{
		Name:        name,
		Description: desc,
		Source:      "import",
		Format:      format,
		Candidates:  candidates,
		CreatedAt:   time.Now().UTC(),
		Seed:        seed,
		Parameters:  params,
		Raw:         primitive.Binary{Subtype: 0x00, Data: b},
		RawFilename: fileHeader.Filename,
		RawMime:     mime,
	}

	res, err := s.db.Collection("datasets").InsertOne(ctx, doc)
	if err != nil {
		return "", "", err
	}
	dsid := res.InsertedID.(primitive.ObjectID)

	if len(parsed.Ballots) > 0 && len(candidates) > 0 {
		cset := map[string]struct{}{}
		for _, c := range candidates {
			id := strings.TrimSpace(c.ID)
			if id != "" {
				cset[id] = struct{}{}
			}
		}

		bdocs := make([]BallotDoc, 0, len(parsed.Ballots))
		for i, it := range parsed.Ballots {
			vref := strings.TrimSpace(it.VoterRef)
			if vref == "" {
				vref = "v" + itoa(i+1)
			}

			bd := BallotDoc{
				DatasetID: dsid,
				VoterRef:  vref,
			}

			switch format {
			case "approval":
				if len(it.Approval) == 0 {
					continue
				}
				seen := map[string]struct{}{}
				for _, cid := range it.Approval {
					cid = strings.TrimSpace(cid)
					if cid == "" {
						continue
					}
					if _, ok := cset[cid]; !ok {
						continue
					}
					if _, ok := seen[cid]; ok {
						continue
					}
					seen[cid] = struct{}{}
					bd.Approval = append(bd.Approval, cid)
				}
				if len(bd.Approval) == 0 {
					continue
				}

			case "ranking":
				if len(it.Ranking) == 0 {
					continue
				}
				seen := map[string]struct{}{}
				for _, cid := range it.Ranking {
					cid = strings.TrimSpace(cid)
					if cid == "" {
						continue
					}
					if _, ok := cset[cid]; !ok {
						continue
					}
					if _, ok := seen[cid]; ok {
						continue
					}
					seen[cid] = struct{}{}
					bd.Ranking = append(bd.Ranking, cid)
				}
				if len(bd.Ranking) == 0 {
					continue
				}

			case "score":
				if len(it.Scores) == 0 {
					continue
				}
				bd.Scores = map[string]int{}
				for cid, v := range it.Scores {
					cid = strings.TrimSpace(cid)
					if cid == "" {
						continue
					}
					if _, ok := cset[cid]; !ok {
						continue
					}
					bd.Scores[cid] = v
				}
				if len(bd.Scores) == 0 {
					continue
				}
			}

			bdocs = append(bdocs, bd)
		}

		if len(bdocs) > 0 {
			_, err = s.db.Collection("dataset_ballots").InsertMany(ctx, toAny(bdocs))
			if err != nil {
				return "", "", err
			}
		}
	}

	return dsid.Hex(), "", nil
}
