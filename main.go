package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/abema/go-mp4"
)

const prefetchKey = "skd://itunes.apple.com/P000000000/s1/e1"

type SampleInfo struct {
	data      []byte
	duration  uint32
	descIndex uint32
}

type SongInfo struct {
	r           io.ReadSeeker
	samples     []SampleInfo
	encaBoxInfo *mp4.BoxInfo
}

func (s *SongInfo) Duration() (ret uint64) {
	for i := range s.samples {
		ret += uint64(s.samples[i].duration)
	}
	return
}

func writeM4a(w *mp4.Writer, info *SongInfo, data []byte) error {
	{ // ftyp
		box, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeFtyp()})
		if err != nil {
			return err
		}
		_, err = mp4.Marshal(w, &mp4.Ftyp{
			MajorBrand:   [4]byte{'M', '4', 'A', ' '},
			MinorVersion: 0,
			CompatibleBrands: []mp4.CompatibleBrandElem{
				{CompatibleBrand: [4]byte{'M', '4', 'A', ' '}},
				{CompatibleBrand: [4]byte{'m', 'p', '4', '2'}},
				{CompatibleBrand: mp4.BrandISOM()},
				{CompatibleBrand: [4]byte{0, 0, 0, 0}},
			},
		}, box.Context)
		if err != nil {
			return err
		}
		_, err = w.EndBox()
		if err != nil {
			return err
		}
	}

	const chunkSize uint32 = 5
	duration := info.Duration()
	numSamples := uint32(len(info.samples))
	var stco *mp4.BoxInfo

	{ // moov
		_, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMoov()})
		if err != nil {
			return err
		}
		box, err := mp4.ExtractBox(info.r, nil, mp4.BoxPath{mp4.BoxTypeMoov()})
		if err != nil {
			return err
		}
		moovOri := box[0]

		{ // mvhd
			_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMvhd()})
			if err != nil {
				return err
			}

			oriBox, err := mp4.ExtractBoxWithPayload(info.r, moovOri, mp4.BoxPath{mp4.BoxTypeMvhd()})
			if err != nil {
				return err
			}
			mvhd := oriBox[0].Payload.(*mp4.Mvhd)
			if mvhd.Version == 0 {
				mvhd.DurationV0 = uint32(duration)
			} else {
				mvhd.DurationV1 = duration
			}

			_, err = mp4.Marshal(w, mvhd, oriBox[0].Info.Context)
			if err != nil {
				return err
			}

			_, err = w.EndBox()
			if err != nil {
				return err
			}
		}

		{ // trak
			_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeTrak()})
			if err != nil {
				return err
			}

			box, err := mp4.ExtractBox(info.r, moovOri, mp4.BoxPath{mp4.BoxTypeTrak()})
			if err != nil {
				return err
			}
			trakOri := box[0]

			{ // tkhd
				_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeTkhd()})
				if err != nil {
					return err
				}

				oriBox, err := mp4.ExtractBoxWithPayload(info.r, trakOri, mp4.BoxPath{mp4.BoxTypeTkhd()})
				if err != nil {
					return err
				}
				tkhd := oriBox[0].Payload.(*mp4.Tkhd)
				if tkhd.Version == 0 {
					tkhd.DurationV0 = uint32(duration)
				} else {
					tkhd.DurationV1 = duration
				}
				tkhd.SetFlags(0x7)

				_, err = mp4.Marshal(w, tkhd, oriBox[0].Info.Context)
				if err != nil {
					return err
				}

				_, err = w.EndBox()
				if err != nil {
					return err
				}
			}

			{ // mdia
				_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMdia()})
				if err != nil {
					return err
				}

				box, err := mp4.ExtractBox(info.r, trakOri, mp4.BoxPath{mp4.BoxTypeMdia()})
				if err != nil {
					return err
				}
				mdiaOri := box[0]

				{ // mdhd
					_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMdhd()})
					if err != nil {
						return err
					}

					oriBox, err := mp4.ExtractBoxWithPayload(info.r, mdiaOri, mp4.BoxPath{mp4.BoxTypeMdhd()})
					if err != nil {
						return err
					}
					mdhd := oriBox[0].Payload.(*mp4.Mdhd)
					if mdhd.Version == 0 {
						mdhd.DurationV0 = uint32(duration)
					} else {
						mdhd.DurationV1 = duration
					}

					_, err = mp4.Marshal(w, mdhd, oriBox[0].Info.Context)
					if err != nil {
						return err
					}

					_, err = w.EndBox()
					if err != nil {
						return err
					}
				}

				{ // hdlr
					oriBox, err := mp4.ExtractBox(info.r, mdiaOri, mp4.BoxPath{mp4.BoxTypeHdlr()})
					if err != nil {
						return err
					}

					err = w.CopyBox(info.r, oriBox[0])
					if err != nil {
						return err
					}
				}

				{ // minf
					_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMinf()})
					if err != nil {
						return err
					}

					box, err := mp4.ExtractBox(info.r, mdiaOri, mp4.BoxPath{mp4.BoxTypeMinf()})
					if err != nil {
						return err
					}
					minfOri := box[0]

					{ // smhd, dinf
						boxes, err := mp4.ExtractBoxes(info.r, minfOri, []mp4.BoxPath{
							{mp4.BoxTypeSmhd()},
							{mp4.BoxTypeDinf()},
						})
						if err != nil {
							return err
						}

						for _, b := range boxes {
							err = w.CopyBox(info.r, b)
							if err != nil {
								return err
							}
						}
					}

					{ // stbl
						_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeStbl()})
						if err != nil {
							return err
						}

						{ // stsd
							box, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeStsd()})
							if err != nil {
								return err
							}
							_, err = mp4.Marshal(w, &mp4.Stsd{EntryCount: 1}, box.Context)
							if err != nil {
								return err
							}

							// For all codecs, copy the original enca box
							err = w.CopyBox(info.r, info.encaBoxInfo)
							if err != nil {
								return err
							}

							_, err = w.EndBox()
							if err != nil {
								return err
							}
						}

						{ // stts
							box, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeStts()})
							if err != nil {
								return err
							}

							var stts mp4.Stts
							for _, sample := range info.samples {
								if len(stts.Entries) != 0 {
									last := &stts.Entries[len(stts.Entries)-1]
									if last.SampleDelta == sample.duration {
										last.SampleCount++
										continue
									}
								}
								stts.Entries = append(stts.Entries, mp4.SttsEntry{
									SampleCount: 1,
									SampleDelta: sample.duration,
								})
							}
							stts.EntryCount = uint32(len(stts.Entries))

							_, err = mp4.Marshal(w, &stts, box.Context)
							if err != nil {
								return err
							}

							_, err = w.EndBox()
							if err != nil {
								return err
							}
						}

						{ // stsc
							box, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeStsc()})
							if err != nil {
								return err
							}

							if numSamples%chunkSize == 0 {
								_, err = mp4.Marshal(w, &mp4.Stsc{
									EntryCount: 1,
									Entries: []mp4.StscEntry{
										{
											FirstChunk:             1,
											SamplesPerChunk:        chunkSize,
											SampleDescriptionIndex: 1,
										},
									},
								}, box.Context)
							} else {
								_, err = mp4.Marshal(w, &mp4.Stsc{
									EntryCount: 2,
									Entries: []mp4.StscEntry{
										{
											FirstChunk:             1,
											SamplesPerChunk:        chunkSize,
											SampleDescriptionIndex: 1,
										}, {
											FirstChunk:             numSamples/chunkSize + 1,
											SamplesPerChunk:        numSamples % chunkSize,
											SampleDescriptionIndex: 1,
										},
									},
								}, box.Context)
							}

							_, err = w.EndBox()
							if err != nil {
								return err
							}
						}

						{ // stsz
							box, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeStsz()})
							if err != nil {
								return err
							}

							stsz := mp4.Stsz{SampleCount: numSamples}
							for _, sample := range info.samples {
								stsz.EntrySize = append(stsz.EntrySize, uint32(len(sample.data)))
							}

							_, err = mp4.Marshal(w, &stsz, box.Context)
							if err != nil {
								return err
							}

							_, err = w.EndBox()
							if err != nil {
								return err
							}
						}

						{ // stco
							box, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeStco()})
							if err != nil {
								return err
							}

							l := (numSamples + chunkSize - 1) / chunkSize
							_, err = mp4.Marshal(w, &mp4.Stco{
								EntryCount:  l,
								ChunkOffset: make([]uint32, l),
							}, box.Context)

							stco, err = w.EndBox()
							if err != nil {
								return err
							}
						}

						_, err = w.EndBox()
						if err != nil {
							return err
						}
					}

					_, err = w.EndBox()
					if err != nil {
						return err
					}
				}

				_, err = w.EndBox()
				if err != nil {
					return err
				}
			}

			_, err = w.EndBox()
			if err != nil {
				return err
			}
		}

		{ // udta
			ctx := mp4.Context{UnderUdta: true}
			_, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeUdta(), Context: ctx})
			if err != nil {
				return err
			}

			{ // meta
				ctx.UnderIlstMeta = true

				_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMeta(), Context: ctx})
				if err != nil {
					return err
				}

				_, err = mp4.Marshal(w, &mp4.Meta{}, ctx)
				if err != nil {
					return err
				}

				{ // hdlr
					_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeHdlr(), Context: ctx})
					if err != nil {
						return err
					}

					_, err = mp4.Marshal(w, &mp4.Hdlr{
						HandlerType: [4]byte{'m', 'd', 'i', 'r'},
						Reserved:    [3]uint32{0x6170706c, 0, 0},
					}, ctx)
					if err != nil {
						return err
					}

					_, err = w.EndBox()
					if err != nil {
						return err
					}
				}

				{ // ilst
					ctx.UnderIlst = true

					_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeIlst(), Context: ctx})
					if err != nil {
						return err
					}

					ctx.UnderIlst = false

					_, err = w.EndBox()
					if err != nil {
						return err
					}
				}

				ctx.UnderIlstMeta = false
				_, err = w.EndBox()
				if err != nil {
					return err
				}
			}

			ctx.UnderUdta = false
			_, err = w.EndBox()
			if err != nil {
				return err
			}
		}

		_, err = w.EndBox()
		if err != nil {
			return err
		}
	}

	{
		box, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMdat()})
		if err != nil {
			return err
		}

		_, err = mp4.Marshal(w, &mp4.Mdat{Data: data}, box.Context)
		if err != nil {
			return err
		}

		mdat, err := w.EndBox()

		var realStco mp4.Stco

		offset := mdat.Offset + mdat.HeaderSize
		for i := uint32(0); i < numSamples; i++ {
			if i%chunkSize == 0 {
				realStco.EntryCount++
				realStco.ChunkOffset = append(realStco.ChunkOffset, uint32(offset))
			}
			offset += uint64(len(info.samples[i].data))
		}

		_, err = stco.SeekToPayload(w)
		if err != nil {
			return err
		}
		_, err = mp4.Marshal(w, &realStco, box.Context)
		if err != nil {
			return err
		}
	}

	return nil
}

func decryptSong(agentIp string, filename string, id string, info *SongInfo, keys []string) error {
	conn, err := net.Dial("tcp", agentIp)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Pre-allocate space for decrypted data
	totalSize := uint64(0)
	for _, sp := range info.samples {
		totalSize += uint64(len(sp.data))
	}
	decrypted := make([]byte, 0, totalSize)

	var lastIndex uint32 = 255 // MaxUint8
	for _, sp := range info.samples {
		if lastIndex != sp.descIndex {
			if len(decrypted) != 0 {
				_, err := conn.Write([]byte{0, 0, 0, 0})
				if err != nil {
					return err
				}
			}
			keyUri := keys[sp.descIndex]

			_, err := conn.Write([]byte{byte(len(id))})
			if err != nil {
				return err
			}
			_, err = io.WriteString(conn, id)
			if err != nil {
				return err
			}

			_, err = conn.Write([]byte{byte(len(keyUri))})
			if err != nil {
				return err
			}
			_, err = io.WriteString(conn, keyUri)
			if err != nil {
				return err
			}
		}
		lastIndex = sp.descIndex

		err := binary.Write(conn, binary.LittleEndian, uint32(len(sp.data)))
		if err != nil {
			return err
		}

		_, err = conn.Write(sp.data)
		if err != nil {
			return err
		}

		de := make([]byte, len(sp.data))
		_, err = io.ReadFull(conn, de)
		if err != nil {
			return err
		}

		decrypted = append(decrypted, de...)
	}
	_, _ = conn.Write([]byte{0, 0, 0, 0, 0})

	create, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer create.Close()

	return writeM4a(mp4.NewWriter(create), info, decrypted)
}

func extractSong(inputPath string) (*SongInfo, error) {
	rawSong, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, err
	}
	f := bytes.NewReader(rawSong)

	trex, err := mp4.ExtractBoxWithPayload(f, nil, []mp4.BoxType{
		mp4.BoxTypeMoov(),
		mp4.BoxTypeMvex(),
		mp4.BoxTypeTrex(),
	})
	if err != nil || len(trex) != 1 {
		return nil, err
	}
	trexPay := trex[0].Payload.(*mp4.Trex)

	stbl, err := mp4.ExtractBox(f, nil, []mp4.BoxType{
		mp4.BoxTypeMoov(),
		mp4.BoxTypeTrak(),
		mp4.BoxTypeMdia(),
		mp4.BoxTypeMinf(),
		mp4.BoxTypeStbl(),
	})
	if err != nil || len(stbl) != 1 {
		return nil, err
	}

	enca, err := mp4.ExtractBoxWithPayload(f, stbl[0], []mp4.BoxType{
		mp4.BoxTypeStsd(),
		mp4.BoxTypeEnca(),
	})
	if err != nil {
		return nil, err
	}

	extracted := &SongInfo{
		r:           f,
		encaBoxInfo: &enca[0].Info,
	}

	moofs, err := mp4.ExtractBox(f, nil, []mp4.BoxType{
		mp4.BoxTypeMoof(),
	})
	if err != nil || len(moofs) <= 0 {
		return nil, err
	}

	mdats, err := mp4.ExtractBoxWithPayload(f, nil, []mp4.BoxType{
		mp4.BoxTypeMdat(),
	})
	if err != nil || len(mdats) != len(moofs) {
		return nil, err
	}

	for i, moof := range moofs {
		tfhd, err := mp4.ExtractBoxWithPayload(f, moof, []mp4.BoxType{
			mp4.BoxTypeTraf(),
			mp4.BoxTypeTfhd(),
		})
		if err != nil || len(tfhd) != 1 {
			return nil, err
		}
		tfhdPay := tfhd[0].Payload.(*mp4.Tfhd)
		index := tfhdPay.SampleDescriptionIndex
		if index != 0 {
			index--
		}

		truns, err := mp4.ExtractBoxWithPayload(f, moof, []mp4.BoxType{
			mp4.BoxTypeTraf(),
			mp4.BoxTypeTrun(),
		})
		if err != nil || len(truns) <= 0 {
			return nil, err
		}

		mdat := mdats[i].Payload.(*mp4.Mdat).Data
		for _, t := range truns {
			for _, en := range t.Payload.(*mp4.Trun).Entries {
				info := SampleInfo{descIndex: index}

				switch {
				case t.Payload.CheckFlag(0x200):
					info.data = mdat[:en.SampleSize]
					mdat = mdat[en.SampleSize:]
				case tfhdPay.CheckFlag(0x10):
					info.data = mdat[:tfhdPay.DefaultSampleSize]
					mdat = mdat[tfhdPay.DefaultSampleSize:]
				default:
					info.data = mdat[:trexPay.DefaultSampleSize]
					mdat = mdat[trexPay.DefaultSampleSize:]
				}

				switch {
				case t.Payload.CheckFlag(0x100):
					info.duration = en.SampleDuration
				case tfhdPay.CheckFlag(0x8):
					info.duration = tfhdPay.DefaultSampleDuration
				default:
					info.duration = trexPay.DefaultSampleDuration
				}

				extracted.samples = append(extracted.samples, info)
			}
		}
		if len(mdat) != 0 {
			return nil, fmt.Errorf("offset mismatch")
		}
	}

	return extracted, nil
}

func main() {
	if len(os.Args) != 6 {
		fmt.Fprintf(os.Stderr, "usage: %s <agentIp> <id> <key> <inputPath> <outputPath>\n", os.Args[0])
		os.Exit(1)
	}

	agentIp := os.Args[1]
	id := os.Args[2]
	key := os.Args[3]
	inputPath := os.Args[4]
	outputPath := os.Args[5]

	info, err := extractSong(inputPath)
	if err != nil {
		panic(err)
	}

	keys := []string{prefetchKey, key}
	err = decryptSong(agentIp, outputPath, id, info, keys)
	if err != nil {
		panic(err)
	}
}
