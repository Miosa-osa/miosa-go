package miosa

import (
	"context"
	"fmt"
)

// ComputerVolumesService manages volume attachments for a computer.
// Accessed via Computer.Volumes.
type ComputerVolumesService struct {
	client     *Client
	computerID string
}

// VolumeAttachmentData is the API representation of a volume attachment.
type VolumeAttachmentData map[string]interface{}

func (s *ComputerVolumesService) base() string {
	return fmt.Sprintf("/computers/%s/volumes", s.computerID)
}

// List returns all volume attachments for the computer.
func (s *ComputerVolumesService) List(ctx context.Context) ([]VolumeAttachmentData, error) {
	var wrapper struct {
		Data        []VolumeAttachmentData `json:"data"`
		Attachments []VolumeAttachmentData `json:"attachments"`
		Volumes     []VolumeAttachmentData `json:"volumes"`
		Items       []VolumeAttachmentData `json:"items"`
	}
	if err := s.client.getJSON(ctx, s.base(), &wrapper); err != nil {
		var list []VolumeAttachmentData
		if err2 := s.client.getJSON(ctx, s.base(), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]VolumeAttachmentData{
		wrapper.Data, wrapper.Attachments, wrapper.Volumes, wrapper.Items,
	} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []VolumeAttachmentData{}, nil
}

// Attach mounts volumeID at mountPath inside the VM.
func (s *ComputerVolumesService) Attach(ctx context.Context, volumeID, mountPath string) (VolumeAttachmentData, error) {
	var out VolumeAttachmentData
	if err := s.client.postJSON(ctx, s.base(),
		map[string]string{"volume_id": volumeID, "mount_path": mountPath},
		&out,
	); err != nil {
		return nil, err
	}
	return out, nil
}

// Detach removes an existing attachment by attachment ID.
func (s *ComputerVolumesService) Detach(ctx context.Context, attachmentID string) error {
	return s.client.deleteJSON(ctx, s.base()+"/"+attachmentID, nil)
}
