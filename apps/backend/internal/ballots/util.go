package ballots

import (
	"crypto/sha256"
	"encoding/hex"
)

func computeVoterHash(electionID, userID string) string {
	h := sha256.Sum256([]byte("election:" + electionID + ":user:" + userID))
	return hex.EncodeToString(h[:])
}

func toJSONBOrNull(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
