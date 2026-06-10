package envelope

type Envelope[T any] struct {
	OK          bool      `json:"ok"`
	ToolVersion string    `json:"tool_version"`
	Data        T         `json:"data"`
	Meta        Meta      `json:"meta"`
	Warnings    []Warning `json:"warnings"`
	Commands    []Command `json:"commands"`
	Errors      []Error   `json:"errors"`
}

type Meta struct {
	RequestID       string  `json:"request_id"`
	TSISO           string  `json:"ts_iso"`
	ContractVersion string  `json:"contract_version"`
	ElapsedMS       int64   `json:"elapsed_ms"`
	DataHash        *string `json:"data_hash"`
	SearchMode      *string `json:"search_mode"`
	FallbackTier    *string `json:"fallback_tier"`
	FallbackReason  *string `json:"fallback_reason"`
}

type Warning struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

type Command struct {
	Label              string  `json:"label"`
	Command            string  `json:"command"`
	Rationale          string  `json:"rationale"`
	AvailableInVersion *string `json:"available_in_version"`
}

type Error struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	DidYouMean *string        `json:"did_you_mean"`
	ExitCode   int            `json:"exit_code"`
	Retryable  bool           `json:"retryable"`
	Details    map[string]any `json:"details"`
}

func New[T any](toolVersion string, data T, opts Options) (Envelope[T], error) {
	meta, err := NewMeta(data, opts)
	if err != nil {
		return Envelope[T]{}, err
	}
	return Envelope[T]{
		OK:          len(opts.Errors) == 0,
		ToolVersion: toolVersion,
		Data:        data,
		Meta:        meta,
		Warnings:    nonNilWarnings(opts.Warnings),
		Commands:    nonNilCommands(opts.Commands),
		Errors:      nonNilErrors(opts.Errors),
	}, nil
}

func nonNilWarnings(in []Warning) []Warning {
	if in == nil {
		return []Warning{}
	}
	return in
}

func nonNilCommands(in []Command) []Command {
	if in == nil {
		return []Command{}
	}
	return in
}

func nonNilErrors(in []Error) []Error {
	if in == nil {
		return []Error{}
	}
	return in
}
