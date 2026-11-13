package simsave

// cardState is the sim's state of a leader or character, with its fields in the order the sim
// writes them. Lists must be empty rather than null.
type cardState struct {
	CardDef                         *struct{}  `json:"cardDef"`
	DeckUniqueID                    int        `json:"deckUniqueID"`
	CardLife                        int        `json:"cardLife"`
	CardPower                       int        `json:"cardPower"`
	CardCombatPower                 int        `json:"cardCombatPower"`
	CardCombatBasePower             int        `json:"cardCombatBasePower"`
	BTapped                         bool       `json:"bTapped"`
	BFaceUp                         bool       `json:"bFaceUp"`
	BSummonSick                     bool       `json:"bSummonSick"`
	BFirstTurn                      bool       `json:"bFirstTurn"`
	BHasBattledCharacter            bool       `json:"bHasBattledCharacter"`
	BOppCharacterKOd                bool       `json:"bOppCharacterKOd"`
	LbActionsUsed                   []bool     `json:"lb_ActionsUsed"`
	BUnblockable                    bool       `json:"bUnblockable"`
	BBanish                         bool       `json:"bBanish"`
	BCantAttack                     bool       `json:"bCantAttack"`
	BCantRest                       bool       `json:"bCantRest"`
	BCanAttackActive                bool       `json:"bCanAttackActive"`
	BDoubleAttack                   bool       `json:"bDoubleAttack"`
	BBlocker                        bool       `json:"bBlocker"`
	BBlockerToOppTurnEnd            bool       `json:"bBlockerToOppTurnEnd"`
	BLoseBlocker                    bool       `json:"bLoseBlocker"`
	BImmune                         bool       `json:"bImmune"`
	ICostChange                     int        `json:"iCostChange"`
	ICostChangeToOppEnd             int        `json:"iCostChangeToOppEnd"`
	BMyCharsImmune                  bool       `json:"bMyCharsImmune"`
	BGLOBALTENACITY                 bool       `json:"bGLOBALTENACITY"`
	BGLOBALEPHEMERAL                bool       `json:"bGLOBALEPHEMERAL"`
	BMyDonCantActivateViaCharacters bool       `json:"bMyDonCantActivateViaCharacters"`
	BCantPlayAnyFromHand            bool       `json:"bCantPlayAnyFromHand"`
	BCantPlayAnyCharactersToField   bool       `json:"bCantPlayAnyCharactersToField"`
	BOpponentLostLife               bool       `json:"bOpponentLostLife"`
	BHandTrashedByEffect            bool       `json:"bHandTrashedByEffect"`
	BMyFieldUnblockable             bool       `json:"bMyFieldUnblockable"`
	CardPowerToStart                int        `json:"cardPowerToStart"`
	BNoDrawFromLifeToStart          bool       `json:"bNoDrawFromLifeToStart"`
	BKinemonLeaderEffect            bool       `json:"bKinemonLeaderEffect"`
	BRosinanteLeaderEffect          bool       `json:"bRosinanteLeaderEffect"`
	BCombatImmune                   bool       `json:"bCombatImmune"`
	BTempBanish                     bool       `json:"bTempBanish"`
	CardTempCombatPower             int        `json:"cardTempCombatPower"`
	BForcedFaceUp                   bool       `json:"bForcedFaceUp"`
	BLastTappedState                bool       `json:"bLastTappedState"`
	BSkipNextActive                 bool       `json:"bSkipNextActive"`
	BEffectImmune                   bool       `json:"bEffectImmune"`
	CardBasePowerChange             int        `json:"cardBasePowerChange"`
	ITurnEndActiveDon               int        `json:"iTurnEndActiveDon"`
	ITurnEndGainActiveDon           int        `json:"iTurnEndGainActiveDon"`
	ICantAttackCostXOrLess          int        `json:"iCantAttackCostXOrLess"`
	ICantPlayOriginalCostOrMore     int        `json:"iCantPlayOriginalCostOrMore"`
	BCantDrawViaEffects             bool       `json:"bCantDrawViaEffects"`
	CardPowerToOwnersTurnEnd        int        `json:"cardPowerToOwnersTurnEnd"`
	CardPowerToOppTurnEnd           int        `json:"cardPowerToOppTurnEnd"`
	IHandUIOrder                    int        `json:"iHandUIOrder"`
	BTakeAnotherTurn                bool       `json:"bTakeAnotherTurn"`
	BTurnEndEqDon                   bool       `json:"bTurnEndEqDon"`
	ITurnEndHeal                    int        `json:"iTurnEndHeal"`
	BTempDoubleAttack               bool       `json:"bTempDoubleAttack"`
	BActivateFilmAtEnd              bool       `json:"bActivateFilmAtEnd"`
	ITurnEndTrashFilm               int        `json:"iTurnEndTrashFilm"`
	CardBasePowerChangeToStart      int        `json:"cardBasePowerChangeToStart"`
	CardBasePowerChangeToOppEnd     int        `json:"cardBasePowerChangeToOppEnd"`
	BFieldCantAttackLeader          bool       `json:"bFieldCantAttackLeader"`
	BCombatImmuneToStart            bool       `json:"bCombatImmuneToStart"`
	BSilenced                       bool       `json:"bSilenced"`
	BSilencedToOwnersEnd            bool       `json:"bSilencedToOwnersEnd"`
	BRogerSilence                   bool       `json:"bRogerSilence"`
	BCombatImmuneToEnd              bool       `json:"bCombatImmuneToEnd"`
	BRushCharacters                 bool       `json:"bRushCharacters"`
	IConfusion                      int        `json:"iConfusion"`
	BNoOnPlayToTurnEnd              bool       `json:"bNoOnPlayToTurnEnd"`
	BUsoppCostChange                bool       `json:"bUsoppCostChange"`
	LGrantedActions                 []struct{} `json:"lGrantedActions"`
	TEndOfTurnActions               []struct{} `json:"tEndOfTurnActions"`
	TStartOfMainPhaseActions        []struct{} `json:"tStartOfMainPhaseActions"`
	LExtraStrikeTypes               []struct{} `json:"lExtraStrikeTypes"`
	IMaxDon                         int        `json:"iMaxDon"`
}

func newCardState() cardState {
	return cardState{
		LbActionsUsed: []bool{false}, LGrantedActions: []struct{}{}, TEndOfTurnActions: []struct{}{},
		TStartOfMainPhaseActions: []struct{}{}, LExtraStrikeTypes: []struct{}{},
	}
}
