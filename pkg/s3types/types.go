// Package s3types provides the S3-compatible XML types used by the sangraha
// API. They are exported so that external tooling can reuse them.
package s3types

import (
	"encoding/xml"
	"time"
)

// ----------------------------------------------------------------------------
// Bucket types
// ----------------------------------------------------------------------------

// Bucket describes a single S3 bucket in a ListBuckets response.
type Bucket struct {
	Name         string    `xml:"Name"`
	CreationDate time.Time `xml:"CreationDate"`
}

// ListAllMyBucketsResult is the root element of a ListBuckets response.
type ListAllMyBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Owner   Owner    `xml:"Owner"`
	Buckets []Bucket `xml:"Buckets>Bucket"`
}

// Owner identifies the owner of a bucket or object.
type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

// CreateBucketConfiguration is the optional request body for CreateBucket.
type CreateBucketConfiguration struct {
	XMLName            xml.Name `xml:"CreateBucketConfiguration"`
	LocationConstraint string   `xml:"LocationConstraint"`
}

// ----------------------------------------------------------------------------
// Object types
// ----------------------------------------------------------------------------

// Object describes a single object in a list response.
type Object struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
	StorageClass string    `xml:"StorageClass"`
	Owner        *Owner    `xml:"Owner,omitempty"`
}

// CommonPrefix holds the common prefix for a grouped listing.
type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

// ListBucketResult is the root element of a ListObjectsV2 response.
type ListBucketResult struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	KeyCount              int            `xml:"KeyCount"`
	MaxKeys               int            `xml:"MaxKeys"`
	Delimiter             string         `xml:"Delimiter,omitempty"`
	IsTruncated           bool           `xml:"IsTruncated"`
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
	StartAfter            string         `xml:"StartAfter,omitempty"`
	Contents              []Object       `xml:"Contents"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes"`
}

// CopyObjectResult is the response body for a successful CopyObject.
type CopyObjectResult struct {
	XMLName      xml.Name  `xml:"CopyObjectResult"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
}

// DeleteObjectsRequest is the request body for a multi-object delete.
type DeleteObjectsRequest struct {
	XMLName xml.Name         `xml:"Delete"`
	Quiet   bool             `xml:"Quiet"`
	Objects []ObjectToDelete `xml:"Object"`
}

// ObjectToDelete identifies an object in a DeleteObjects request.
type ObjectToDelete struct {
	Key       string `xml:"Key"`
	VersionID string `xml:"VersionId,omitempty"`
}

// DeleteResult is the response body for a DeleteObjects operation.
type DeleteResult struct {
	XMLName xml.Name      `xml:"DeleteResult"`
	Deleted []Deleted     `xml:"Deleted"`
	Errors  []DeleteError `xml:"Error"`
}

// Deleted reports a successfully deleted object.
type Deleted struct {
	Key       string `xml:"Key"`
	VersionID string `xml:"VersionId,omitempty"`
}

// DeleteError reports a failed deletion within a DeleteObjects response.
type DeleteError struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// ----------------------------------------------------------------------------
// Multipart upload types
// ----------------------------------------------------------------------------

// InitiateMultipartUploadResult is the response to CreateMultipartUpload.
type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}

// CompleteMultipartUpload is the request body for CompleteMultipartUpload.
type CompleteMultipartUpload struct {
	XMLName xml.Name       `xml:"CompleteMultipartUpload"`
	Parts   []CompletePart `xml:"Part"`
}

// CompletePart identifies a single part in a CompleteMultipartUpload request.
type CompletePart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// CompleteMultipartUploadResult is the response to CompleteMultipartUpload.
type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

// Part describes an uploaded part in a ListParts response.
type Part struct {
	PartNumber   int       `xml:"PartNumber"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
}

// ListPartsResult is the response to ListParts.
type ListPartsResult struct {
	XMLName              xml.Name `xml:"ListPartsResult"`
	Bucket               string   `xml:"Bucket"`
	Key                  string   `xml:"Key"`
	UploadID             string   `xml:"UploadId"`
	PartNumberMarker     int      `xml:"PartNumberMarker"`
	NextPartNumberMarker int      `xml:"NextPartNumberMarker,omitempty"`
	MaxParts             int      `xml:"MaxParts"`
	IsTruncated          bool     `xml:"IsTruncated"`
	Parts                []Part   `xml:"Part"`
}

// ListMultipartUploadsResult is the response to ListMultipartUploads.
type ListMultipartUploadsResult struct {
	XMLName   xml.Name          `xml:"ListMultipartUploadsResult"`
	Bucket    string            `xml:"Bucket"`
	Prefix    string            `xml:"Prefix,omitempty"`
	Delimiter string            `xml:"Delimiter,omitempty"`
	Uploads   []MultipartUpload `xml:"Upload"`
}

// MultipartUpload describes an in-progress multipart upload.
type MultipartUpload struct {
	Key       string    `xml:"Key"`
	UploadID  string    `xml:"UploadId"`
	Initiated time.Time `xml:"Initiated"`
}

// ----------------------------------------------------------------------------
// Error type
// ----------------------------------------------------------------------------

// ErrorResponse is the S3-compatible XML error response returned on failure.
type ErrorResponse struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestID string   `xml:"RequestId,omitempty"`
}
