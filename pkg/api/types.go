package api

type ValidatorEpochStats struct {
	Data []struct {
		Cluster                string  `json:"cluster"`
		Epoch                  int     `json:"epoch"`
		AgaveMinVersion        string  `json:"agave_min_version"`
		AgaveMaxVersion        *string `json:"agave_max_version"`
		FiredancerMaxVersion   *string `json:"firedancer_max_version"`
		FiredancerMinVersion   string  `json:"firedancer_min_version"`
		InheritedFromPrevEpoch bool    `json:"inherited_from_prev_epoch"`
	} `json:"data"`
}
