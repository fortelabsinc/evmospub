package types

// ValidateBasic is used for validating the packet
func (p OrderPacketData) ValidateBasic() error {

	// TODO: Validate the packet data

	return nil
}

// GetBytes is a helper for serialising
func (p OrderPacketData) GetBytes() ([]byte, error) {
	var modulePacket CustomtransferPacketData

	modulePacket.Packet = &CustomtransferPacketData_OrderPacket{&p}

	return modulePacket.Marshal()
}
