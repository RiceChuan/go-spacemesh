// Code generated by github.com/spacemeshos/go-scale/scalegen. DO NOT EDIT.

// nolint
package wire

import (
	"github.com/spacemeshos/go-scale"
	"github.com/spacemeshos/go-spacemesh/common/types"
)

func (t *MarryProof) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.MarriageCertificatesRoot[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.MarriageCertificatesProof, 32)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := t.Certificate.EncodeScale(enc)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.CertificateProof, 32)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.CertificateIndex))
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *MarryProof) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.MarriageCertificatesRoot[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeStructSliceWithLimit[types.Hash32](dec, 32)
		if err != nil {
			return total, err
		}
		total += n
		t.MarriageCertificatesProof = field
	}
	{
		n, err := t.Certificate.DecodeScale(dec)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeStructSliceWithLimit[types.Hash32](dec, 32)
		if err != nil {
			return total, err
		}
		total += n
		t.CertificateProof = field
	}
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.CertificateIndex = uint32(field)
	}
	return total, nil
}

func (t *MarriageProof) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.MarriageATX[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.MarriageATXProof, 32)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.MarriageATXSmesherID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := t.NodeIDMarryProof.EncodeScale(enc)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := t.SmesherIDMarryProof.EncodeScale(enc)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *MarriageProof) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.MarriageATX[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeStructSliceWithLimit[types.Hash32](dec, 32)
		if err != nil {
			return total, err
		}
		total += n
		t.MarriageATXProof = field
	}
	{
		n, err := scale.DecodeByteArray(dec, t.MarriageATXSmesherID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := t.NodeIDMarryProof.DecodeScale(dec)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := t.SmesherIDMarryProof.DecodeScale(dec)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
