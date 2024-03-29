package vdfloc

// Publicly available high level functions

import (
	"fmt"
	"github.com/fabdem/go-vdfloc/config"
	"regexp"
	"strings"
)

// type t_PluralGender struct {
// 	suffix	string
// 	check	interface{}
// 	}

var m_pluralGender map[string]interface{}

// var suffixesPluralGender []string
var pluralTag string
var genderTags []string
var allTags []string
var conf *config.Config

const defaultJson = "pluralgender.json" // located along with the exe or bin

func init() {

	// Defines each token suffixe and its associated check function
	m_pluralGender = map[string]interface{}{
		":p":  checkPlural,               // plural
		":n":  checkGenderSender,         // gender sender
		":g":  checkGenderReceiver,       // gender receiver
		":np": checkGenderSenderPlural,   // gender sender with plural
		":gp": checkGenderReceiverPlural, // gender receiver with plural
	}

	genderTags = []string{
		"#|f|#",
		"#|n|#",
		"#|c|#",
		"#|m|#",
		"#|ma|#",
		"#|mi|#",
		"#|mp|#",
	}

	pluralTag = "#|#"

	allTags = append(genderTags, pluralTag)

	// Try to load the default config file
	conf, _ = config.New(defaultJson)
}

// LoadJsonConf()
//
// Load a json config file.
// 	Input:
//		- path and name or nil if default
// 	Output:
//		- err != nil if error
//		- update global var conf
//
func LoadJsonConf(f string) (err error) {

	conf, err = config.New(f)

	return err
}

// checkPlural()
//
// Check plural syntax in a token value.
// 	Input:
//		- token name
//		- token value
//		- Language name
// 	Output:
//		- issue == nil if no syntax issue
//		- err
//
func checkPlural(k string, v string, lang string) (res string, err error) {
	n, err := conf.GetPlural(lang)
	if err != nil {
		return res, err
	}

	if n > 0 {
		n--
	} // e.g. 2 form plural -> 1 separator

	if ct := strings.Count(v, pluralTag); ct != n {
		res = fmt.Sprintf("Expected number of plural forms: %d - found: %d", n+1, ct+1)
	}
	return res, err
}

// checkGenderSender()
//
// Check gender syntax in a sender token value. Needs either 1 of tag list for that language.
// 	Input:
//		- token name
//		- token value
//		- Language name
// 	Output:
//		- issue == nil if no syntax issue
//		- err
//
func checkGenderSender(k string, v string, lang string) (res string, err error) {
	l, err := conf.GetGenders(lang)
	if err != nil {
		return res, err
	}

	var list string // Convert slice to a single string
	for _, val := range l {
		list += val + ","
	}

	var total int

	for _, gender := range genderTags {

		ct := strings.Count(v, gender)

		if ok := strings.Contains(list, gender); (ct > 1) || (ct == 1 && !ok) { // bad syntax cases
			if len(list) > 0 {
				res = fmt.Sprintf("Error with gender form: %s - expected only one of: %s", gender, list)
			} else {
				res = fmt.Sprintf("Error with gender form: %s - no gender expected", gender)
			}
			break
		} else {
			if ct > 0 { // found one good match
				total++
			}
		}
	}

	if len(l) > 0 && total != 1 { // If we have not found exactly 1 match when there are genders
		res = fmt.Sprintf("Error with gender form - expected %s", list)
	}

	return res, err
}

// checkGenderReceiver()
//
// Check gender syntax in a receiver token value. Needs 1 of each tag for that language.
// 	Input:
//		- token name
//		- token value
//		- Language name
// 	Output:
//		- issue == nil if no syntax issue
//		- err
//
func checkGenderReceiver(k string, v string, lang string) (res string, err error) {
	l, err := conf.GetGenders(lang)
	if err != nil {
		return res, err
	}

	var list string // Convert slice to a single string
	for _, val := range l {
		list += (val + ",")
	}

	var total int
	minIdx := 32767 // Looks for the 1st tag's position (initial value arbitrarily high)

	for _, gender := range genderTags { // check all gender tag possible

		ct := strings.Count(v, gender)

		if ok := strings.Contains(list, gender); (ct != 1 || !ok) && (ct != 0 || ok) { // bad syntax cases
			if len(list) > 0 {
				res = fmt.Sprintf("Error with gender form: %s - expected one of each: %s", gender, list)
			} else {
				res = fmt.Sprintf("Error with gender form: %s - no gender expected", gender)
			}
			break
		} else { // (ct == 1 && ok) || (ct ==0 && !ok)
			if ok && ct == 1 {
				total++
				if idx := strings.Index(v, gender); idx < minIdx {
					minIdx = idx
				}
			}
		}
	}

	if total != len(l) { // If we don't have one of each -> syntax problem
		res = fmt.Sprintf("Error with gender form - expected %s", list)
	}

	if len(l) > 1 && minIdx > 0 { // The 1st gender tag needs to be at idx 0 otherwise syntax err
		res = fmt.Sprintf("Error with gender form - the first gender tag should be at the begining of the string. Found at position %d", minIdx)
	}

	return res, err
}

// checkGenderSenderPlural()
//
// Check gender syntax in a sender token value with plural. Needs as many gender
// tags valid for the language as they are plurals.
// If there are no genders but plurals (e.g. schinese) plurals are separated with the plural tag.
// 	Input:
//		- token name
//		- token value
//		- Language name
// 	Output:
//		- issue == nil if no syntax issue
//		- err
//
//	E.g. "Valve_TestPluralGenders_Noun1:np"    "#|m|#Trésor#|m|#Trésors"
//
func checkGenderSenderPlural(k string, v string, lang string) (res string, err error) {
	l, err := conf.GetGenders(lang) // Get the list of gender tags
	if err != nil {
		return res, err
	}

	nbPluralExpected, err := conf.GetPlural(lang) // Get the number of plurals
	if err != nil {
		return res, err
	}

	if nbPluralExpected > 0 && len(l) == 0 {
		// Exception: if plurals but no gender: form separator is the one used for plurals
		nbPluralExpected-- // e.g. 2 form plural -> 1 separator

		if ct := strings.Count(v, pluralTag); ct != nbPluralExpected {
			res = fmt.Sprintf("Error with gender/plural form: found %d plural forms, while expecting %d separated wiht a  plural tag.", ct+1, nbPluralExpected+1)
			return res, err // Syntax issue detected
		}
	} else {

		var list string // Convert slice into a single string
		for _, val := range l {
			list += val + ","
		}

		pluralCount := 0

		for _, gender := range genderTags {
			if ct := strings.Count(v, gender); ct > 0 && !strings.Contains(list, gender) {
				res = fmt.Sprintf("Error with gender/plural form: this tag was unexpected %s", gender)
				break
			} else {
				pluralCount += ct
			}
		}

		if pluralCount != nbPluralExpected { // If incorrect number of plural forms ->  error
			res = fmt.Sprintf("Error with gender/plural forms - counted %d while expecting %d", pluralCount, nbPluralExpected)
		}
	}
	return res, err
}

// checkGenderReceiverplural()
//
// Check gender syntax in a receiver token value with plural.
// Each gender list must be repeated as many time as there are plurals for the language.
// If there are no genders but plurals (e.g. schinese) plurals are separated with the plural tag.
// 	Input:
//		- token name
//		- token value
//		- Language name
// 	Output:
//		- issue	== nil if no syntax issue
//		- err	!= nil is processing error
//
// E.g. "Valve_TestPluralGenders_Adjective1:gp" "#|m|#peu Commun#|f|#peu Commune#|m|#peu Communs#|f|#peu Communes"
//
func checkGenderReceiverPlural(k string, v string, lang string) (res string, err error) {
	lgGenderTags, err := conf.GetGenders(lang) // Get the list of gender tags
	if err != nil {
		return res, err // Processing error
	}

	var list string // Convert slice to a single string
	for _, val := range lgGenderTags {
		list += (val + ",")
	}

	nbPluralExpected, err := conf.GetPlural(lang) // Get the number of plurals
	if err != nil {
		return res, err // Processing error
	}

	if nbPluralExpected > 0 && len(lgGenderTags) == 0 {
		// Exception: if plurals but no gender: form separator is the one used for plurals
		nbPluralExpected-- // e.g. 2 form plural -> 1 separator

		if ct := strings.Count(v, pluralTag); ct != nbPluralExpected {
			res = fmt.Sprintf("Error with gender/plural form: found %d plural forms, while expecting %d separated wiht a  plural tag.", ct+1, nbPluralExpected+1)
			return res, err // Syntax issue detected
		}

	} else {
		// 1st - check presence of the right tags and the right number of times
		//       and build an array of tag indexes in the token string for later order check.

		arrayIdx := make([][]int, nbPluralExpected+1)
		for i := range arrayIdx {
			arrayIdx[i] = make([]int, len(lgGenderTags)+2)
		}

		g := 1
		for _, gender := range genderTags {
			ct := strings.Count(v, gender)
			if ok := strings.Contains(list, gender); (ct != nbPluralExpected || !ok) && (ct != 0 || ok) {
				// bad syntax cases: wrong tag present or correct tag but wrong number of instances
				if len(list) > 0 {
					res = fmt.Sprintf("Error with gender/plural form: %s - found %d plural forms while expecting %d of each gender group: %s", ct, gender, nbPluralExpected, list)
				} else {
					res = fmt.Sprintf("Error with gender/plural form: %s - no gender expected", gender) // No gender expected but found gender tags...
				}
				return res, err // Syntax issue detected
			} else {
				if ok {
					// If tag valid for this language
					// Let's capture the position of each tag in the string
					str := v
					for p := 1; p <= nbPluralExpected; p++ {
						idx := strings.Index(str, gender)
						arrayIdx[p][g] = idx + (len(v) - len(str))
						str = str[idx+len(gender):]
					}
					g++ // next column
				}
			}
		}

		// 2nd - check that the tags are in the right order
		// 		 We already checked that the right number of the right gender tags are in there.
		// 		 Tags must be organised in as many groups as there are plurals. Each group must have all
		// 		 gender tags (no specific order required).

		for p := 1; p <= nbPluralExpected; p++ {
			for g := 1; g <= len(lgGenderTags); g++ {
				if arrayIdx[p][g] < arrayIdx[p-1][len(lgGenderTags)+1] {
					// Error order incorrect. Provides pointer to where the error is.
					res = fmt.Sprintf("Error with gender/plural form: incorrect order plural form: %d, gender tag: %s", p, lgGenderTags[g-1])
					return res, err // Syntax issue detected
				}
				if arrayIdx[p][g] > arrayIdx[p][len(lgGenderTags)+1] {
					arrayIdx[p][len(lgGenderTags)+1] = arrayIdx[p][g] // keep track of highest index
				}
			}
		}
	}
	return res, err
}

// FilterPlrGdr()
//
// Keeps only plural and gender tokens.
// 	Input:
//		- Slice of token names
// 	Output:
//		- Slice of plural and gender token names
//
func (v *VDFFile) FilterPlrGdr(in []string) (out []string) {
	v.log(fmt.Sprintf("FilterPlrGdr()"))

	var isKeyPlrExtForm = regexp.MustCompile(`:p\{[a-zA-Z_\d:]+\}$`).MatchString // capture the 'p:{value_name}' form

	for _, tkn := range in {
		for sufx, _ := range m_pluralGender {
			if strings.HasSuffix(tkn, sufx) || isKeyPlrExtForm(tkn) {
				out = append(out, tkn)
				break
			}
		}
	}
	return out
}

// checkNonPlrlGdr()
//
// Check that the value of a non plural/gender key doesn't contain
// plural separators or gender tags.
// 	Input:
//		- token value
// 	Output:
//		- issue == nil if no syntax issue
//		- err != nil if processing error
//
func (v *VDFFile) CheckNonPlrlGdr(key string, val string) (issue string, err error) {
	// v.log(fmt.Sprintf("CheckNonPlrlGdr(%s, %s)", key, val)) remove log out of concerns about performance impact

	for _, tag := range allTags {
		if strings.Index(val, tag) != -1 {
			issue = fmt.Sprintf("Error - found plural separators and/or gender tags (%s) in a non gendered/plural token: %s - %s", tag, key, val)
			break
		}
	}
	return issue, err
}

// CheckPlrlGendrTokenVal()
//
// Check plural and gender syntax of a token value.
// If it's not a plural or gender token just ignore (return nil string)
// unless there are plural separators or gender tags in the value.
// 	Input:
//		- token name
//		- token value
//		- Language name
// 	Output:
//		- issue == nil if no syntax issue or not a gender/plural variant
//		- err
//
func (v *VDFFile) CheckPlrlGendrTokenVal(token string, val string, language string) (issue string, err error) {
	v.log(fmt.Sprintf("CheckPlrlGendrTokenVal(%s, %s, %s)", token, val, language))

	// Capture tag (:p, :n, :g, :gp, etc.) and call the right function to check syntax
	if capturedTag := regexp.MustCompile(`(:[png]{1,2})(?:\{[a-zA-Z_\d:]+\})?$`).FindStringSubmatch(token); len(capturedTag) > 1 {

		if f, ok := m_pluralGender[capturedTag[1]]; ok {
			issue, err = f.(func(string, string, string) (string, error))(token, val, language) // Check syntax
			// bOK,bArrayRes := record.fctOpen.(func (string) (bool,[]byte))(openingTag)
		}
	}
	
	return issue, err
}
