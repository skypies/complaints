package complaintdb

import(
	"encoding/gob"
	"io"

	"github.com/skypies/util/date"
)

func (cdb ComplaintDB)MarshalComplaintSlice(complaints []Complaint, w io.Writer) error {
	return gob.NewEncoder(w).Encode(complaints)
}

func (cdb ComplaintDB)UnmarshalComplaintSlice(r io.Reader) ([]Complaint, error) {
	complaints := []Complaint{}

	if err := gob.NewDecoder(r).Decode(&complaints); err != nil {
		return nil, err
	}

	// Need a few cleanups on the persisted data. It would be better to
	// perform these during writing, not reading, but that would mean
	// mutating the original data during save, esp. the DatastoreKey,
	// which could prove a surprise.
	for i,_ := range complaints {
		complaints[i] = complaints[i].ToCopyWithStoredDataOnly()	// Blank out synthetic fields; we shouldn't really have stored them
		complaints[i].Timestamp = date.InPdt(complaints[i].Timestamp) // time.GobDecode messes with timezones

		if complaints[i].Submission.Response == nil {
			complaints[i].Submission.Response = []byte{}  // Gob also maps empty slices to the nil value ?
		}
	}

	return complaints, nil
}
