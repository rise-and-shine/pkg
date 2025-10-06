package types

// ImageSize represents the size of an image
type ImageSize string

// Predefined image size labels used for naming and categorizing resized images.
//
// These constants represent the available image sizes supported by the service.
// They are used throughout the image processing pipeline and storage operations.
//
// - Original: the unmodified, full-resolution image.
// - Medium:   a resized version suitable for standard displays or previews.
// - Small:    a smaller thumbnail-size version suitable for avatars or listings.
const (
	Original ImageSize = "original"
	Medium   ImageSize = "medium"
	Small    ImageSize = "small"
)
