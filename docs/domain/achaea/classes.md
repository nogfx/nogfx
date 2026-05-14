# Classes

Achaea has twenty-one classes. Each class is defined by its three **skill sets** (sometimes called "skills" for short — the lesson/rank progression they share is on [`skills.md`](skills.md)). A character belongs to exactly one class at a time; switching is possible but expensive.

Skill descriptions below are one-line paraphrases of in-game help files. Consult `HELP <SKILL>` in-game or the [Achaea wiki](https://wiki.achaea.com/) for canonical wording.

## The classes

### Alchemist
Enigmatic figures wielding the power of the ether.
- **Alchemy** — transmutation of base materials and the brewing of potent reagents.
- **Physiology** — manipulation of one's own physical form, including stoneskin and bodily resilience.
- **Formulation** (Sublimation in some cities) — combat application of alchemical compounds.

### Apostate
Necromantic daemon summoners aligned with Evil.
- **Evileye** — gaze-based dark magic.
- **Necromancy** — manipulation of life-force and the dead.
- **Apostasy** — daemon summoning and binding.

### Bard
Swashbuckling troubadour adventurers.
- **Bladedance** — flowing rapier-and-cape swordplay.
- **Composition** — musical magic that buffs allies and harms enemies.
- **Sagas** (or **Woe** for Cyrenian Bards) — chanted battle-poems with combat effects.

### Blademaster
Masters of the legendary Two Arts.
- **TwoArts** — dual-blade combat with two specialised swords.
- **Striking** — precision strikes targeting body parts.
- **Shindo** — meditative discipline that powers combat techniques.

### Depthswalker
Manipulators of shadows and time.
- **Aeonics** — time manipulation and temporal effects.
- **Shadowmancy** — shadow-based magic and stealth.
- **Terminus** — the magic of endings, death, and finality.

### Druid
Forest-loving metamorphs.
- **Groves** — ritual magic centered on the druid's sacred grove.
- **Metamorphosis** — shapeshifting into forest beasts.
- **Reclamation** — magic that turns civilisation back into wilderness.

### Infernal
Evil knights employing necromantic magic.
- **Weaponmastery** — broad mastery of swords, axes, maces, and the like.
- **Oppression** — dark abilities that subjugate and torment.
- **Malignity** — necromantic combat techniques.

### Jester
Happy-go-lucky pranksters and roguish entertainers.
- **Tarot** — arcane cards with major-arcana effects from the Sun to Death.
- **Pranks** — joke-themed combat abilities that hit harder than they sound.
- **Puppetry** — voodoo-style control through crafted puppets.

### Magi
Masters of the four elements and crystalline vibrations.
- **Elementalism** — summoning and commanding elemental creatures.
- **Crystalism** — crystal-focused magic, including the crystal lattice.
- **Artificing** — crafting magical artefacts.

### Monk
Forges mind, body, and spirit into a unified whole.
- **Tekura** (or **Shikudo**) — striking arts whose damage and speed depend on the active stance.
- **Kaido** — defensive bodily discipline; higher tiers use Kai energy built up through combat.
- **Telepathy** — mental manipulation of opponents.

### Occultist
Chaos-loving summoners of extra-planar entities.
- **Occultism** — manipulation of auras, karma, and the bending of time and reality.
- **Tarot** — arcane cards (see Jester).
- **Domination** — summoning Chaos entities and Chaos Lords; the most dangerous art in Achaea.

### Paladin
Knights of Good wielding devotion alongside their weapons.
- **Weaponmastery** — broad mastery of conventional weapons.
- **Excision** — holy combat techniques targeting evil.
- **Valour** — honour-based abilities that empower the paladin in righteous combat.

### Pariah
Exiled mages working with blood, insects, and an abhorrence of death.
- **Memorium** — manipulation of memory and identity.
- **Pestilence** — insect-borne disease and swarming attacks.
- **Charnel** — magic centred on decay and the corpse.

### Priest
Holy warriors with a fearsome guardian angel.
- **Spirituality** — divine magic drawing on faith.
- **Devotion** — summoning and directing a guardian angel.
- **Zeal** — fervour-based combat enhancements.

### Psion
Weavers and emulators using stolen Aldar power from Saar-elan.
- **Weaving** — manipulation of psionic threads.
- **Psionics** — mental attacks and defences.
- **Emulation** — mimicking the abilities of other classes.

### Runewarden
Mystic knights employing runic lore alongside their swords.
- **Weaponmastery** — broad weapon mastery.
- **Runelore** — scribing and invoking runes for combat and utility.
- **Discipline** — knightly training that enhances endurance and resolve.

### Sentinel
Metamorphing forest skirmishers with animal companions.
- **Metamorphosis** — shapeshifting into beasts (see Druid).
- **Woodlore** — knowledge and use of the forest as a weapon.
- **Skirmishing** — hit-and-run forest combat with animal allies.

### Serpent
Masters of venoms, hypnosis, and subterfuge — perhaps the trickiest class.
- **Subterfuge** — stealth, lockpicking, and dirty fighting.
- **Venom** — poison creation and delivery.
- **Hypnosis** — implanting suggestions that fire later in combat.

### Shaman
Mystical spirit channellers supplementing their powers with Vodun dolls and curses.
- **Spiritlore** — communication with and binding of spirits.
- **Curses** — laying lasting hexes on enemies.
- **Vodun** — voodoo dolls that bind target and shaman together.

### Sylvan
Forest practitioners aligned with the wild.
- **Propagation** — accelerating the growth and spread of forest.
- **Groves** — ritual magic in the sylvan's grove (see Druid).
- **Weatherweaving** — calling and controlling weather as a weapon.

### Unnamable
Combat-focused warriors embracing destruction and chaos.
- **Weaponmastery** — broad weapon mastery.
- **Anathema** — speaking forbidden truths and curses.
- **Dominion** — exerting destructive control over a region.

## How nogfx uses class information

The world layer maintains a `Character` struct (`pkg/world/achaea/character.go`) that tracks class via GMCP — primarily `Char.Name` and `Char.Status`. Class-aware features (custom prompts, balance tracking per skill, bashing helpers) hang off this.

## References

- [Classes | Achaea](https://www.achaea.com/classes)
- [Category:Classes — AchaeaWiki](https://wiki.achaea.com/Category:Classes)
- [Category:Skills — AchaeaWiki](https://wiki.achaea.com/Category:Skills)
