/*
Copyright 2020 The pdf Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pdfcpu

import (
	"encoding/hex"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
)

var ErrHasAcroForm = errors.New("pdfcpu: existing")

type Signer interface {
	EstimateSignatureLength() int
	Sign(data io.Reader) ([]byte, error)
}

// Sign creates a digital signature for xRefTable and writes the result to outFile.
func (ctx *Context) Sign(outFile string, sigDict *Dict, signer Signer) error {
	// hack
	ctx.Configuration.WriteObjectStream = false

	maxSigContentBytes := signer.EstimateSignatureLength()

	// Write to outFile
	ctx.Write.DirName = "."
	ctx.Write.FileName = outFile
	if err := Write(ctx); err != nil {
		return err
	}

	a0 := 0
	a1 := int(ctx.Write.OffsetSigContents)
	a2 := int(ctx.Write.OffsetSigContents) + 2 + maxSigContentBytes*2
	a3 := int(ctx.Write.FileSize)

	_ = ctx.Write.FileSize
	_ = ctx.Write.OffsetSigByteRange
	_ = ctx.Write.OffsetSigContents

	a := NewIntegerArray(
		a0,
		a1-a0,
		a2,
		a3-a2,
	)

	// Patch "ByteArray" in signature dict.
	if err := patchFile(outFile, []byte(a.PDFString()), ctx.Write.OffsetSigByteRange); err != nil {
		return err
	}

	// Read hashed part of file
	f, err := os.OpenFile(outFile, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	part0 := io.NewSectionReader(f, int64(a0), int64(a1-a0))
	part1 := io.NewSectionReader(f, int64(a2), int64(a3-a2))

	pdfReader := io.MultiReader(part0, part1)

	// Create signature(outFile, byteRanges)
	sig, err := signer.Sign(pdfReader)
	if err != nil {
		return err
	}
	bb := []byte(hex.EncodeToString(sig))

	// Patch "Contents" in signature dict.
	return patchFile(outFile, bb, ctx.Write.OffsetSigContents+1)
}

func patchFile(fileName string, bb []byte, offset int64) error {
	f, err := os.OpenFile(fileName, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	if _, err := f.WriteAt(bb, offset); err != nil {
		return err
	}

	return nil
}

func (ctx *Context) PrepareSignature(signer Signer) (*Dict, error) {
	// hack
	ctx.Configuration.WriteObjectStream = false

	xRefTable := ctx.XRefTable

	rootDict, err := xRefTable.Catalog()
	if err != nil {
		return nil, err
	}

	if _, found := rootDict.Find("AcroForm"); found {
		return nil, ErrHasAcroForm
	}

	// Create Sig dict:
	// <ByteRange, [0 59646 64308 4898]> reserve 60 Bytes within  []
	// <Contents, <3082059506092A864886F70D010702A082058630820582020101310F300D06096086480165030402010500300B06092A864886F70D010701A08203643082036030820248A003020102020AE74317AF54536B3AA019300D06092A864886F70D01010B0500305E311530130603550403130C486F7273742052757474657231093007060355040A130031093007060355040B13003122302006092A864886F70D01090116136868727574746572407064666370752E636F6D310B3009060355040613025553301E170D3230303632383231313534315A170D3235303632383231313534315A305E311530130603550403130C486F7273742052757474657231093007060355040A130031093007060355040B13003122302006092A864886F70D01090116136868727574746572407064666370752E636F6D310B300906035504061302555330820122300D06092A864886F70D01010105000382010F003082010A02820101009AF53F756A6F9749E3AA5F3F51019BA3925A31B0069E4FA7C2CBE0CE35BB053D917E696714382A685073E19B5EB35EFDC6645D7C8F17817FBC9765D1DFA2E96AADBE9C8D7994217377A9DB42BD7220B92A4156D240FA0C8F389225B08543143892F597324F795A90563BA00BA21A2E925E3F78AD4ACB5E493E954797B11E63F60B0C319699A5CC5DE14E2C8641B8782D01D815992460FCFAB54603794CB767AE944E92737F97F87E7DB27A549BB47170CA933FBB422C81CAAE6A87EA0030B8D784D6B85E07D40BF52DE6697646AA44A355B40818430D02718E8BC0BA01E4C3AF61F78BB787F77D0C7DC4511F939C333F105A660F60D40B639B406A813C645D8B0203010001A320301E300F06092A864886F72F01010A04020500300B0603551D0F040403020398300D06092A864886F70D01010B050003820101008F178EE065B0AF5A547AF4456FFC079C157C42A98A05D60F4D964D1B786CCA7F9F2DA287B53DCA55962722433797DEA425795789AC94274C4DDAA380B627A7231BC0D4118AED53025F9DF121ACA5778F65DFCEF8A2060B1810A977C128773BF87670E6B6E19A3B864F20E79327D65E3A7B4C53424634DC7AB268D7A01A72CD5611868230EABFAA251ACB79EE866A767A03E30BAC0192B723C75F23FFFCD5BC2DF8A3D84C28F47DA7422AF4B849A8E29533FE2D3836FCE7E9C71934F7BCA13379E3198A5358E0B3EBE9B280E796AD4E2322978183A298B48BA948F1223B1A9A1E167E26B6655F5BC2E3D4C72627E068AE54D70A119AE5E2D2667FA1A5F68AAB85318201F5308201F1020101306C305E311530130603550403130C486F7273742052757474657231093007060355040A130031093007060355040B13003122302006092A864886F70D01090116136868727574746572407064666370752E636F6D310B3009060355040613025553020AE74317AF54536B3AA019300D06096086480165030402010500A05C300F06092A864886F72F01010831023000301806092A864886F70D010903310B06092A864886F70D010701302F06092A864886F70D0109043122042028F79D34E143D24A3FC50C553A16D05F49B8FCABBDF13E5C0675655D29D55AA4300D06092A864886F70D01010B05000482010015D4B5CA272503FB0C7EFC6B2C79F9B7CDE6CD3A67E5D079B74587BB8F710148661FC53D4DCD91B7797E25C1C3488C9B28D592F1C9E085B4F3CE170AFC022B08E90FC8113DCAE7502FE600F635C02784A90AAD44FD3E8B6C59EEDED1A271ACCE044F92B483A65DBCC1E658622B73CA705CA5A206738BDC78E8E240E826E343E13BE8D91E3D25CC9B6179483E54078ADD2E3407E7B9F9E72665E79CDC4CDADF29BBC56396072063EE6143CD9C2417588F4EDDB420726F81044035D53F41C909117DF9687FCBFC45FA6C7F5BE883EA027B3DF603333A1438CBEB98649A6876822A1019248DBB55099D24D0D3F5E2CE2655FABF5944C54315020EE4035D6657C454000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000>>
	// <Filter,  Adobe.PPKLite>
	// <M, (D:20200629000822+02'00')>
	// <Name, (Horst Rutter)>
	// <SubFilter, adbe.pkcs7.detached>
	// <Type, Sig>

	maxSigContentBytes := signer.EstimateSignatureLength()

	sigDict := Dict(
		map[string]Object{
			"Type":      Name("Sig"),
			"Filter":    Name("Adobe.PPKLite"),
			"SubFilter": Name("adbe.pkcs7.detached"),
			"Contents":  NewHexLiteral(make([]byte, maxSigContentBytes)),
			"ByteRange": Array{},
			"M":         StringLiteral(DateString(time.Now())),
		},
	)

	ir, err := xRefTable.IndRefForNewObject(sigDict)
	if err != nil {
		return nil, err
	}

	// Create Acrofield
	// 25: was compressed 19[2] generation=0 pdfcpu.Dict type=Annot subType=Widget
	// <<
	// 	<F, 132>
	// 	<FT, Sig>
	// 	<MK, <<
	// 	>>>
	// 	<P, (13 0 R)>
	// 	<Rect, [151.84 253.47 246.53 347.75]>
	// 	<Subtype, Widget>
	// 	<T, (Signature2)>
	// 	<Type, Annot>
	// 	<V, (21 0 R)>
	// >>

	sigFieldDict := Dict(
		map[string]Object{
			"Type":    Name("Annot"),
			"Subtype": Name("Widget"),
			"FT":      Name("Sig"),
			"T":       StringLiteral("Signature"),
			"Rect":    NewNumberArray(0, 0, 0, 0),
			"V":       *ir,
		},
	)

	if ir, err = xRefTable.IndRefForNewObject(sigFieldDict); err != nil {
		return nil, err
	}

	// Link 1st page to Signature Field dictionary, otherwise signature is not visible. check specs...
	pg, _ := xRefTable.DereferenceDictEntry(xRefTable.RootDict, "Pages")
	pgd := pg.(Dict)
	kids := pgd.ArrayEntry("Kids")
	p0, _ := xRefTable.DereferenceDict(kids[0])

	annots := Array{*ir}
	p0.Update("Annots", annots)

	// Create AcroForm

	// 	23: was compressed 19[0] generation=0 pdfcpu.Dict
	// <<
	// 	<Fields, [(25 0 R)]>
	// 	<SigFlags, 3>
	// >>

	formDict := Dict(
		map[string]Object{
			"Fields":   Array{*ir},
			"SigFlags": Integer(3),
		},
	)

	if ir, err = xRefTable.IndRefForNewObject(formDict); err != nil {
		return nil, err
	}

	rootDict.Insert("AcroForm", *ir)

	return &sigDict, nil
}

func (ctx *Context) PrepareTimestamp(signer Signer) (*Dict, error) {
	// hack
	ctx.Configuration.WriteObjectStream = false

	xRefTable := ctx.XRefTable

	rootDict, err := xRefTable.Catalog()
	if err != nil {
		return nil, err
	}

	if _, found := rootDict.Find("AcroForm"); found {
		return nil, ErrHasAcroForm
	}

	// Create Sig dict:
	// <ByteRange, [0 59646 64308 4898]> reserve 60 Bytes within  []
	// <Contents, <00000>>
	// <Filter,  Adobe.PPKLite>
	// <SubFilter, ETSI.RFC3161>
	// <Type, DocTimeStamp>
	// <V, 0>

	maxSigContentBytes := signer.EstimateSignatureLength()

	sigDict := Dict(
		map[string]Object{
			"Type":      Name("DocTimeStamp"),
			"Filter":    Name("Adobe.PPKLite"),
			"SubFilter": Name("ETSI.RFC3161"),
			"Contents":  NewHexLiteral(make([]byte, maxSigContentBytes)),
			"ByteRange": Array{},
		},
	)

	ir, err := xRefTable.IndRefForNewObject(sigDict)
	if err != nil {
		return nil, err
	}

	// Create Acrofield
	// 25: was compressed 19[2] generation=0 pdfcpu.Dict type=Annot subType=Widget
	// <<
	// 	<F, 132>
	// 	<FT, Sig>
	// 	<MK, <<
	// 	>>>
	// 	<P, (13 0 R)>
	// 	<Rect, [151.84 253.47 246.53 347.75]>
	// 	<Subtype, Widget>
	// 	<T, (Signature2)>
	// 	<Type, Annot>
	// 	<V, (21 0 R)>
	// >>

	sigFieldDict := Dict(
		map[string]Object{
			"Type":    Name("Annot"),
			"Subtype": Name("Widget"),
			"FT":      Name("Sig"),
			"T":       StringLiteral("Signature"),
			"Rect":    NewNumberArray(0, 0, 0, 0),
			"V":       *ir,
		},
	)

	if ir, err = xRefTable.IndRefForNewObject(sigFieldDict); err != nil {
		return nil, err
	}

	// Link 1st page to Signature Field dictionary, otherwise signature is not visible. check specs...
	pg, _ := xRefTable.DereferenceDictEntry(xRefTable.RootDict, "Pages")
	pgd := pg.(Dict)
	kids := pgd.ArrayEntry("Kids")
	p0, _ := xRefTable.DereferenceDict(kids[0])

	annots := Array{*ir}
	p0.Update("Annots", annots)

	// Create AcroForm

	// 	23: was compressed 19[0] generation=0 pdfcpu.Dict
	// <<
	// 	<Fields, [(25 0 R)]>
	// 	<SigFlags, 3>
	// >>

	formDict := Dict(
		map[string]Object{
			"Fields":   Array{*ir},
			"SigFlags": Integer(3),
		},
	)

	if ir, err = xRefTable.IndRefForNewObject(formDict); err != nil {
		return nil, err
	}

	rootDict.Insert("AcroForm", *ir)

	return &sigDict, nil
}
