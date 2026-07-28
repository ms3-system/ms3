package api

type Object struct {
	Data     ObjectData
	Metadata ObjectMetadata
}

// ObjectData should have a DataDB that reads from the write-ahead log File
type ObjectData struct {
	Data   []byte
	Offset int
	// LogPath is the write-ahead log file path that we will write to on disk.
	LogPath string
	Size    int
}

// ObjectMetadata should represent a row in the Metadata DB
type ObjectMetadata struct {
	ID         string
	Name       string
	BucketID   string
	BucketName string
	ObjectHash []byte
	// ObjectPath is used as the Path for the object we'll store
	// It's supposed to read its contents into Data and write it to Disk.
	// TODO: What if we need to send data directly?
	ObjectPath string
}
