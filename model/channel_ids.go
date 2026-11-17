package model

// GetChannelsByIds returns the channels with the given ids; ids with no channel are left out.
func GetChannelsByIds(ids []int) ([]*Channel, error) {
	var channels []*Channel
	err := DB.Where("id IN ?", ids).Find(&channels).Error
	return channels, err
}
