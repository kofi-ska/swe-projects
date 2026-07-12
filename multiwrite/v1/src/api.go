package multiwrite

import internal "multiwrite/src/internal/multiwrite"

const (
	WriterDirect   = internal.WriterDirect
	WriterBuffered = internal.WriterBuffered

	ProfileFixed       = internal.ProfileFixed
	ProfileTiny        = internal.ProfileTiny
	ProfileWide        = internal.ProfileWide
	ProfileHeavyTail   = internal.ProfileHeavyTail
	ProfileHighEntropy = internal.ProfileHighEntropy
	ProfileLowEntropy  = internal.ProfileLowEntropy
	ProfileBurst       = internal.ProfileBurst

	DurabilityClosed    = internal.DurabilityClosed
	DurabilitySynced    = internal.DurabilitySynced
	DurabilityCommitted = internal.DurabilityCommitted

	OverwriteFail    = internal.OverwriteFail
	OverwriteReplace = internal.OverwriteReplace
	OverwriteVersion = internal.OverwriteVersion

	DefaultBatchSize = internal.DefaultBatchSize

	ManifestSchemaVersion = internal.ManifestSchemaVersion

	ReasonRecordsExhausted     = internal.ReasonRecordsExhausted
	ReasonByteLimitReached     = internal.ReasonByteLimitReached
	ReasonDurationLimitReached = internal.ReasonDurationLimitReached
	ReasonValidationFailed     = internal.ReasonValidationFailed
	ReasonWriterCreateFailed   = internal.ReasonWriterCreateFailed
	ReasonWriteFailed          = internal.ReasonWriteFailed
	ReasonFlushCloseFailed     = internal.ReasonFlushCloseFailed
	ReasonCommitFailed         = internal.ReasonCommitFailed
	ReasonManifestWriteFailed  = internal.ReasonManifestWriteFailed
)

var (
	ErrValidation     = internal.ErrValidation
	ErrFilesystem     = internal.ErrFilesystem
	ErrTimeout        = internal.ErrTimeout
	ErrResourceLimit  = internal.ErrResourceLimit
	ErrApplicationBug = internal.ErrApplicationBug
)

type RunConfig = internal.RunConfig
type RunResult = internal.RunResult
type RunEnvironment = internal.RunEnvironment
type RecordGenerator = internal.RecordGenerator
type MeasuredWriter = internal.MeasuredWriter
type DirectFileWriter = internal.DirectFileWriter
type BufferedFileWriter = internal.BufferedFileWriter

func DefaultRunConfig() RunConfig {
	return internal.DefaultRunConfig()
}

func Run(cfg RunConfig) (RunResult, error) {
	return internal.Run(cfg)
}

func NewRecordGenerator(profile string, seed int64, recordSize int) *RecordGenerator {
	return internal.NewRecordGenerator(profile, seed, recordSize)
}

func NewDirectFileWriter(path string) (*DirectFileWriter, error) {
	return internal.NewDirectFileWriter(path)
}

func NewBufferedFileWriter(path string, bufferSize int) (*BufferedFileWriter, error) {
	return internal.NewBufferedFileWriter(path, bufferSize)
}
