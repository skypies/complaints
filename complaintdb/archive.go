package complaintdb

import(
	"encoding/gob"
	"io"

	"github.com/skypies/complaints/complaintdb/types"
)

func (cdb ComplaintDB)MarshalComplaintSlice(complaints []types.Complaint, w io.Writer) error {
	return gob.NewEncoder(w).Encode(complaints)
}

func (cdb ComplaintDB)UnmarshalComplaintSlice(r io.Reader) ([]types.Complaint, error) {
	complaints := []types.Complaint{}

	if err := gob.NewDecoder(r).Decode(&complaints); err != nil {
		return nil, err
	}

	// Blank out synthetic fields; we shouldn't have stored them
	for i,_ := range complaints {
		complaints[i].DatastoreKey = ""
		complaints[i].Dist2KM = 0.0
		complaints[i].Dist3KM = 0.0
	}

	return complaints, nil
}
