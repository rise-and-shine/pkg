package filestore

// Common MIME content types for file operations.
const (
	// Images.
	ContentTypeJPEG = "image/jpeg"
	ContentTypePNG  = "image/png"
	ContentTypeGIF  = "image/gif"
	ContentTypeWebP = "image/webp"
	ContentTypeSVG  = "image/svg+xml"
	ContentTypeBMP  = "image/bmp"
	ContentTypeICO  = "image/x-icon"
	ContentTypeTIFF = "image/tiff"

	// Audio.
	ContentTypeMP3  = "audio/mpeg"
	ContentTypeWAV  = "audio/wav"
	ContentTypeOGG  = "audio/ogg"
	ContentTypeAAC  = "audio/aac"
	ContentTypeFLAC = "audio/flac"
	ContentTypeWebM = "audio/webm"

	// Video.
	ContentTypeMP4       = "video/mp4"
	ContentTypeAVI       = "video/x-msvideo"
	ContentTypeMOV       = "video/quicktime"
	ContentTypeWMV       = "video/x-ms-wmv"
	ContentTypeVideoOGG  = "video/ogg"
	ContentTypeVideoWebM = "video/webm"
	ContentTypeMKV       = "video/x-matroska"

	// Documents.
	ContentTypePDF  = "application/pdf"
	ContentTypeRTF  = "application/rtf"
	ContentTypeText = "text/plain"
	ContentTypeHTML = "text/html"
	ContentTypeCSS  = "text/css"
	ContentTypeJS   = "application/javascript"
	ContentTypeJSON = "application/json"
	ContentTypeXML  = "application/xml"

	// Microsoft Office.
	ContentTypeDOC  = "application/msword"
	ContentTypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	ContentTypeXLS  = "application/vnd.ms-excel"
	ContentTypeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	ContentTypePPT  = "application/vnd.ms-powerpoint"
	ContentTypePPTX = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

	// OpenDocument.
	ContentTypeODT = "application/vnd.oasis.opendocument.text"
	ContentTypeODS = "application/vnd.oasis.opendocument.spreadsheet"
	ContentTypeODP = "application/vnd.oasis.opendocument.presentation"

	// Archives.
	ContentTypeZIP  = "application/zip"
	ContentTypeRAR  = "application/vnd.rar"
	ContentType7Z   = "application/x-7z-compressed"
	ContentTypeTAR  = "application/x-tar"
	ContentTypeGZIP = "application/gzip"

	// Other.
	ContentTypeOctetStream = "application/octet-stream"
	ContentTypeCSV         = "text/csv"
)
