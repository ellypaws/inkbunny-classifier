package api

import (
	"encoding/json"
	"fmt"
)

type ErrorResponse struct {
	Code    *int   `json:"error_code,omitempty"`
	Message string `json:"error_message"`
}

func CheckError(body []byte) error {
	var response ErrorResponse
	_ = json.Unmarshal(body, &response)

	if response.Code != nil {
		return fmt.Errorf("[%d]: %s", *response.Code, response.Message)
	}

	return nil
}

func Error(body []byte) error {
	var response ErrorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return err
	}

	return response
}

func (error ErrorResponse) Error() string {
	return error.Message
}

const (
	ErrInvalidLogin                      = iota // Invalid login. Username and password incorrect or account does not have API Access enabled in account Settings.
	ErrEmptySessionID                           // No Session ID sent as variable 'sid'. If this error appears then a valid session ID is required as part of the query, but it was not received by the script. Session Ids are obtained by logging in using the api_login.php Login script.
	ErrInvalidSessionID                         // Invalid Session ID sent as variable 'sid'. This error will appear if you send a Session ID (sid) that is not valid, has been logged out or has expired.
	ErrInvalidResultsID                         // Invalid Results ID sent as variable 'rid'. It contains invalid characters. Results Ids can only contain Hexadecimal values.
	ErrNoResultsFound                           // No results found for Results ID sent as variable 'rid'. The Results ID (rid) sent has either expired or is not valid. Results sets will automatically be removed after not being accessed for a period of time, or when a user has created too many results sets (in which case the oldest results sets will be removed first).
	ErrNoPermissionToUpload                     // Current user does not have permission to upload files.
	ErrHourlyLimitReached                       // Submission Hourly Limit Reached. This policy exists to prevent spamming and flooding. We apologise for the inconvenience. Please wait a while and try again to add more uploads.
	ErrDatabaseError                            // Database error. Unable to create a new submission.
	ErrNoValidSubmissionID                      // No valid submission id given.
	ErrNoPermissionToEditSubmission             // Current user does not have permission to edit this submission.
	ErrNoPermissionToEditFile                   // Current user does not have permission to edit this file.
	ErrCouldNotCreateEntry                      // Could not create an entry for the file (filename) in our database. Please try again. If the problem persists, contact an administrator.
	ErrNoPermissionToBulkUpload                 // User does not have permission to BULK upload multiple pages/files at once.
	ErrZIPFileTooBig                            // ZIP file is too big.
	ErrInvalidFileName                          // Incoming file names cannot contain a double-dot '..'. Please rename your file and try again.
	ErrInvalidCharactersInFilenames             // Invalid characters detected in filenames inside your zip file. The server said (error message).
	ErrCouldNotExtractFiles                     // Could not extract any files from that ZIP. ZIP files cannot have subdirectories in them for Bulk Upload. Please check the zip file is not damaged and that it has files in it. If you are sure your zip file is fine, please contact an administrator and tell them about this message.
	ErrNotZIPFile                               // The file you uploaded was not a ZIP file. It was of a non-ZIP file type (type). Please try again with a valid ZIP file. If the problem persists, contact an administrator.
	ErrZIPUploadFailed                          // ZIP upload failed. You might not have provided a file, or the file may have been too big for the size restrictions. If you are sure the file is fine, then the server may be out of space or the tmp uploads directory is not writable. Please contact a system administrator and tell them about this message if you are sure the problem is on our end.
	ErrFileCouldNotBeRead                       // File could not be read. File name was (file name).
	ErrFileTooLarge                             // The file you uploaded (file name) was too large in file size. Please try again with a smaller file size. If the problem persists, contact an administrator.
	ErrUnsupportedFileType                      // The file you uploaded (file name) was of an unsupported file type (type). Please try again with a supported file type as listed. If the problem persists, contact an administrator.
	ErrFileTooLargeInPixelSize                  // The file you uploaded (file name) was too large in pixel size (width and/or height). Please try again with dimensions not exceeding those listed. If the problem persists, contact an administrator.
	ErrFileNotInRGBOrGreyscale                  // The file you uploaded (file name) was not in RGB or Greyscale color mode (most likely it was in CMYK mode). Please try again with an RGB or Greyscale image. If the problem persists, contact an administrator.
	ErrUnknownErrorCheckingFile                 // There was an unknown error when trying to check your uploaded file (file name). Please check your file meets all the listed requirements and try again. If the problem persists, contact an administrator.
	ErrUnsupportedThumbnailType                 // The thumbnail you uploaded (file name) was of an unsupported file type (type). Please try again with a supported thumbnail type as listed. If the problem persists, contact an administrator.
	ErrThumbnailTooLargeInPixelSize             // The thumbnail you uploaded (file name) was too large in pixel size (width and/or height). Please try again with dimensions not exceeding those listed. If the problem persists, contact an administrator.
	ErrThumbnailNotInRGBOrGreyscale             // The thumbnail you uploaded (file name) was not in RGB or Greyscale color mode (most likely it was in CMYK mode). Please try again with an RGB or Greyscale thumbnail. If the problem persists, contact an administrator.
	ErrUnknownErrorCheckingThumbnail            // There was an unknown error when trying to check your uploaded thumbnail (file name). Please check your thumbnail meets all the listed requirements and try again. If the problem persists, contact an administrator.
	ErrTooManySubmissionIDs                     // If you upload all the files in this ZIP file, you will exceed the maximum limit of files/pages per submission. None of the files from your zip were added. Please upload less pages in the one zip file or start a new submission for the remaining pages.
	ErrCancellationRequest                      // Received cancellation request or didn't receive response from browser within timeout limit. Some files may have been uploaded successfully. Stopped uploading on file (file name). That file and any after it in the zip were not added.
	ErrMaxAllowedNumberOfFiles                  // You have reached the maximum allowed number of files/pages for this submission. Stopped uploading on file (file name). That file and any after it in the zip were not added.
	ErrCouldNotUploadThumbnail                  // Could not upload the thumbnail (file name). Please try again. If the problem persists, contact an administrator.
	ErrCouldNotCreateCopyOfFile                 // Could not create copy of that file in our system. Please try again. If the file is a PNG, make sure it is in RGB color mode and not Indexed color. If the problem persists, contact an administrator.
	ErrCouldNotCreateThumbnail                  // Could not create a thumbnail for the file (file name). Thumbnail file was called (thumbnail file name). If the file is a PNG, make sure it is in RGB color mode and not Indexed color. Please try again. If the problem persists, contact an administrator.
	ErrUserCanceled                             // User canceled.
	ErrInvalidProgressKey                       // Invalid Progress Key.
	ErrNoPermissionToDeleteSubmission           // Current user does not have permission to delete this submission.
	ErrNoPermissionToRemoveFile                 // Current user does not have permission to remove this file.
	ErrNoPermissionToChangeOrderOfFile          // Current user does not have permission to change the order of this file.
	ErrSubmissionDeleted                        // That submission has been deleted.
	ErrTooManySubmissionIDsToQuery              // Too many submission ids to query. Limit exceeded.
	ErrNoPermissionToGetFavlist                 // Current user does not have permission to get the favlist of this submission.
	ErrCouldNotCreateZIPtmpExtractionDir        // Couldn't create zip tmp extraction dir.
	ErrCouldNotRenameFileInUnzipProcess         // Could not rename file in unzip process.
	ErrInvalidKeywordID                         // Invalid Keyword ID.
	ErrCouldNotReplaceFileOrThumbnail           // Could not replace that file or thumbnail.
	ErrRequestNotSentInHTTPSMode         = 999  // Request was not sent in HTTPS mode.
)
