package blob

import "github.com/minio/minio-go/v7"

// minioGetMarkerOptions returns the minio GetObjectOptions used when
// reading the claim marker. Kept in a helper file so the imports for
// minio-go stay localized.
func minioGetMarkerOptions() minio.GetObjectOptions {
	return minio.GetObjectOptions{}
}

// minioListOptions returns the minio ListObjectsOptions used by
// Provision's bucket-content scan. Limits the listing to one key so
// the call returns as soon as a single matching object appears.
func minioListOptions(prefix string, maxKeys int) minio.ListObjectsOptions {
	return minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
		MaxKeys:   maxKeys,
	}
}
