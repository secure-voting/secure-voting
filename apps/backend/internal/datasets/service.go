package datasets

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	db *mongo.Database
}

func NewService(db *mongo.Database) *Service {
	return &Service{db: db}
}

type Candidate struct {
	ID   string `json:"id" bson:"id"`
	Name string `json:"name" bson:"name"`
}

type Dataset struct {
	ID          string      `json:"id" bson:"-"`
	Name        string      `json:"name" bson:"name"`
	Description string      `json:"description,omitempty" bson:"description,omitempty"`
	Source      string      `json:"source" bson:"source"`
	Format      string      `json:"format" bson:"format"`
	Candidates  []Candidate `json:"candidates" bson:"candidates"`
	CreatedAt   string      `json:"created_at" bson:"-"`
	Seed        *int64      `json:"seed,omitempty" bson:"seed,omitempty"`
}

type ListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Format    string `json:"format"`
	CreatedAt string `json:"created_at"`
}

type GenerateReq struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Format      string      `json:"format"`
	Candidates  []Candidate `json:"candidates"`
	Voters      int         `json:"voters"`
	Seed        *int64      `json:"seed,omitempty"`

	// для синтетики
	ApprovalMaxChoices *int `json:"approval_max_choices,omitempty"`
	RankingTopK        *int `json:"ranking_top_k,omitempty"`
	ScoreMin           *int `json:"score_min,omitempty"`
	ScoreMax           *int `json:"score_max,omitempty"`
	ScoreStep          *int `json:"score_step,omitempty"`
}

type ImportMeta struct {
	Name        string
	Description string
	Format      string
}

type DatasetDoc struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Name        string             `bson:"name"`
	Description string             `bson:"description,omitempty"`
	Source      string             `bson:"source"`
	Format      string             `bson:"format"`
	Candidates  []Candidate        `bson:"candidates"`
	CreatedAt   time.Time          `bson:"created_at"`
	Seed        *int64             `bson:"seed,omitempty"`

	Raw         primitive.Binary `bson:"raw,omitempty"`
	RawFilename string           `bson:"raw_filename,omitempty"`
	RawMime     string           `bson:"raw_mime,omitempty"`
}

type BallotDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	DatasetID primitive.ObjectID `bson:"dataset_id"`
	Approval  []string           `bson:"approval,omitempty"`
	Ranking   []string           `bson:"ranking,omitempty"`
	Scores    map[string]int     `bson:"scores,omitempty"`
	VoterRef  string             `bson:"voter_ref,omitempty"`
}

func (s *Service) List(ctx context.Context) ([]ListItem, error) {
	coll := s.db.Collection("datasets")
	cur, err := coll.Find(ctx, bson.M{}, nil)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []ListItem
	for cur.Next(ctx) {
		var d DatasetDoc
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, ListItem{
			ID:        d.ID.Hex(),
			Name:      d.Name,
			Source:    d.Source,
			Format:    d.Format,
			CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (Dataset, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return Dataset{}, "invalid_id", nil
	}

	var d DatasetDoc
	err = s.db.Collection("datasets").FindOne(ctx, bson.M{"_id": oid}).Decode(&d)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Dataset{}, "not_found", nil
		}
		return Dataset{}, "", err
	}

	return Dataset{
		ID:          d.ID.Hex(),
		Name:        d.Name,
		Description: d.Description,
		Source:      d.Source,
		Format:      d.Format,
		Candidates:  d.Candidates,
		CreatedAt:   d.CreatedAt.UTC().Format(time.RFC3339),
		Seed:        d.Seed,
	}, "", nil
}

func (s *Service) Download(ctx context.Context, id string) ([]byte, string, string, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return nil, "", "", "invalid_id", nil
	}

	var d DatasetDoc
	err = s.db.Collection("datasets").FindOne(ctx, bson.M{"_id": oid}).Decode(&d)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, "", "", "not_found", nil
		}
		return nil, "", "", "", err
	}

	if len(d.Raw.Data) == 0 {
		// если raw не сохранен — вернем минимальный JSON метаданных
		meta := map[string]any{
			"id":          d.ID.Hex(),
			"name":        d.Name,
			"description": d.Description,
			"source":      d.Source,
			"format":      d.Format,
			"candidates":  d.Candidates,
			"created_at":  d.CreatedAt.UTC().Format(time.RFC3339),
			"seed":        d.Seed,
		}
		b, _ := json.Marshal(meta)
		return b, "dataset.json", "application/json", "", nil
	}

	fn := d.RawFilename
	if fn == "" {
		fn = "dataset.bin"
	}
	ct := d.RawMime
	if ct == "" {
		ct = "application/octet-stream"
	}
	return d.Raw.Data, fn, ct, "", nil
}

func (s *Service) Import(ctx context.Context, meta ImportMeta, fileHeader *multipart.FileHeader, file multipart.File) (string, string, error) {
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		return "", "invalid_name", nil
	}
	format := strings.TrimSpace(meta.Format)
	if format == "" {
		format = "external"
	}

	// читаем файл в память (ограничение на уровне handler)
	b, err := io.ReadAll(file)
	if err != nil {
		return "", "", err
	}

	mime := fileHeader.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	doc := DatasetDoc{
		Name:        name,
		Description: strings.TrimSpace(meta.Description),
		Source:      "import",
		Format:      format,
		Candidates:  []Candidate{},
		CreatedAt:   time.Now().UTC(),
		Raw:         primitive.Binary{Subtype: 0x00, Data: b},
		RawFilename: fileHeader.Filename,
		RawMime:     mime,
	}

	res, err := s.db.Collection("datasets").InsertOne(ctx, doc)
	if err != nil {
		return "", "", err
	}

	oid := res.InsertedID.(primitive.ObjectID)
	return oid.Hex(), "", nil
}

func (s *Service) Generate(ctx context.Context, req GenerateReq) (string, string, error) {
	if strings.TrimSpace(req.Name) == "" {
		return "", "invalid_name", nil
	}
	switch req.Format {
	case "approval", "ranking", "score":
	default:
		return "", "invalid_format", nil
	}
	if len(req.Candidates) < 2 {
		return "", "candidates_required", nil
	}
	if req.Voters <= 0 || req.Voters > 2000 {
		return "", "invalid_voters", nil
	}

	seed := req.Seed
	if seed == nil {
		// псевдо-seed из rand
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		v := int64(0)
		for i := 0; i < 8; i++ {
			v = (v << 8) | int64(b[i])
		}
		seed = &v
	}

	// генерим бюллетени
	dsDoc := DatasetDoc{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Source:      "generate",
		Format:      req.Format,
		Candidates:  req.Candidates,
		CreatedAt:   time.Now().UTC(),
		Seed:        seed,
	}

	ins, err := s.db.Collection("datasets").InsertOne(ctx, dsDoc)
	if err != nil {
		return "", "", err
	}
	dsid := ins.InsertedID.(primitive.ObjectID)

	// детерминированный PRNG (простая LCG, достаточно для синтетики)
	rng := newLCG(uint64(*seed))

	var ballots []BallotDoc
	cids := make([]string, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		cids = append(cids, c.ID)
	}

	for i := 0; i < req.Voters; i++ {
		b := BallotDoc{
			DatasetID: dsid,
			VoterRef:  "v" + itoa(i+1),
		}
		switch req.Format {
		case "approval":
			q := 1
			if req.ApprovalMaxChoices != nil && *req.ApprovalMaxChoices > 0 {
				q = *req.ApprovalMaxChoices
			} else if q > len(cids) {
				q = len(cids)
			}
			k := 1 + int(rng.next()%uint64(q))
			b.Approval = pickSubset(rng, cids, k)

		case "ranking":
			top := len(cids)
			if req.RankingTopK != nil && *req.RankingTopK > 0 && *req.RankingTopK < top {
				top = *req.RankingTopK
			}
			sh := shuffle(rng, cids)
			b.Ranking = sh[:top]

		case "score":
			if req.ScoreMin == nil || req.ScoreMax == nil || req.ScoreStep == nil || *req.ScoreStep <= 0 {
				return "", "score_rules_missing", nil
			}
			b.Scores = map[string]int{}
			for _, id := range cids {
				steps := ((*req.ScoreMax - *req.ScoreMin) / *req.ScoreStep) + 1
				v := int(rng.next()%uint64(steps))
				b.Scores[id] = *req.ScoreMin + v*(*req.ScoreStep)
			}
		}
		ballots = append(ballots, b)
	}

	if len(ballots) > 0 {
		_, err = s.db.Collection("dataset_ballots").InsertMany(ctx, toAny(ballots))
		if err != nil {
			return "", "", err
		}
	}

	// сохраняем raw json для download (ограничение voters <= 2000 делает это безопасным)
	export := map[string]any{
		"dataset": map[string]any{
			"id":          dsid.Hex(),
			"name":        dsDoc.Name,
			"description": dsDoc.Description,
			"source":      dsDoc.Source,
			"format":      dsDoc.Format,
			"candidates":  dsDoc.Candidates,
			"created_at":  dsDoc.CreatedAt.UTC().Format(time.RFC3339),
			"seed":        dsDoc.Seed,
		},
		"ballots": ballotsToJSON(ballots),
	}
	raw, _ := json.Marshal(export)

	_, err = s.db.Collection("datasets").UpdateOne(ctx,
		bson.M{"_id": dsid},
		bson.M{"$set": bson.M{
			"raw":          primitive.Binary{Subtype: 0x00, Data: raw},
			"raw_filename": "dataset.json",
			"raw_mime":     "application/json",
		}},
	)
	if err != nil {
		return "", "", err
	}

	return dsid.Hex(), "", nil
}

func ballotsToJSON(in []BallotDoc) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, b := range in {
		m := map[string]any{
			"voter_ref": b.VoterRef,
		}
		if len(b.Approval) > 0 {
			m["approval"] = b.Approval
		}
		if len(b.Ranking) > 0 {
			m["ranking"] = b.Ranking
		}
		if len(b.Scores) > 0 {
			m["scores"] = b.Scores
		}
		out = append(out, m)
	}
	return out
}

func toAny[T any](in []T) []any {
	out := make([]any, 0, len(in))
	for i := range in {
		out = append(out, in[i])
	}
	return out
}

type lcg struct{ s uint64 }

func newLCG(seed uint64) *lcg { return &lcg{s: seed} }
func (r *lcg) next() uint64 {
	// Numerical Recipes LCG
	r.s = r.s*1664525 + 1013904223
	return r.s
}

func shuffle(r *lcg, in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	for i := len(out) - 1; i > 0; i-- {
		j := int(r.next() % uint64(i+1))
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func pickSubset(r *lcg, in []string, k int) []string {
	if k <= 0 {
		return []string{}
	}
	if k >= len(in) {
		out := make([]string, len(in))
		copy(out, in)
		return out
	}
	sh := shuffle(r, in)
	return sh[:k]
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [32]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + (i % 10))
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

var _ = errors.Is
