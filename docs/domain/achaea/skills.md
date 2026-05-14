# Skills, lessons, and ranks

Every class in Achaea has three **skill sets** ([`classes.md`](classes.md)). Each skill set is independently levelled through twelve **ranks**, paid for with **lessons**. This page covers the universal progression system; per-class skill triples live with the classes.

## The twelve ranks

In order from worst to best:

1. **Inept**
2. **Novice**
3. **Apprentice**
4. **Capable**
5. **Adept**
6. **Skilled**
7. **Gifted**
8. **Expert**
9. **Virtuoso**
10. **Fabled**
11. **Mythical**
12. **Transcendent** (sometimes shortened to **Trans**)

A skill at Transcendent has every ability available; the rank is the cap. "Trans" or "transing" a skill is the common short form for reaching Transcendent.

## Lesson costs

Each rank-up costs a fixed number of lessons. The cost climbs steeply at higher ranks.

| Progression | Lessons (incremental) | Lessons (cumulative) | Credits (cumulative) |
| --- | --- | --- | --- |
| Inept → Novice | 7 | 7 | 2 |
| Novice → Apprentice | 9 | 16 | 3 |
| Apprentice → Capable | 18 | 34 | 6 |
| Capable → Adept | 56 | 90 | 15 |
| Adept → Skilled | 81 | 171 | 29 |
| Skilled → Gifted | 111 | 282 | 47 |
| Gifted → Expert | 149 | 431 | 72 |
| Expert → Virtuoso | 188 | 619 | 104 |
| Virtuoso → Fabled | 315 | 934 | 156 |
| Fabled → Mythical | 383 | 1317 | 220 |
| Mythical → Transcendent | 450 | 1736 | 290 |

Costs are per skill set — trans-ing all three of a class's skill sets is roughly 5,200 lessons. Iron Realms has periodically rebalanced this curve; treat these numbers as the current snapshot, not eternal truth.

## Earning lessons

Two paths:

- **Levelling**. Each new experience level grants a fixed bundle of lessons. This is the free route.
- **Credits → lessons**. Bound credits can be converted to lessons at a fixed rate (currently ~6 lessons per credit). Credits are bought with real money or earned in-game through quests, the trading post, and IRE's various bonus systems.

`SHOW LESSONS LEFT IN <skill>` reports how many lessons remain until Transcendent in that skill set.

## What the codebase models

`pkg/gmcp/charskills.go` decodes the `Char.Skills.*` family of GMCP messages:

- `Char.Skills.Groups` — list of skill sets the character has, with rank.
- `Char.Skills.List` — abilities within a given skill set.
- `Char.Skills.Info` — detail on a specific ability.
- `Char.Skills.Get` — client request for one of the above.

`pkg/world/achaea/learning.go` is the in-flight piece that consumes these — see the [refactor status](../../design/refactor-status.md) before assuming any specific behaviour.

## References

- [Skill ranks — AchaeaWiki](https://wiki.achaea.com/Skill_ranks)
- [Lessons — AchaeaWiki](https://wiki.achaea.com/Lessons)
- [Category:Skills — AchaeaWiki](https://wiki.achaea.com/Category:Skills)
