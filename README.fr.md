<div align="center">

# keypulse

### Statistiques clavier et souris dans votre propre base de séries temporelles

Touches, clics, molette, distance de la souris, répartition par application et heatmaps —
envoyés à Telegraf ou InfluxDB, affichés dans Grafana. Comme WhatPulse, sans le cloud.

[![Release](https://img.shields.io/github/v/release/jonathan-gtd/keypulse?color=41BDF5)](https://github.com/jonathan-gtd/keypulse/releases/latest) [![CI](https://img.shields.io/github/actions/workflow/status/jonathan-gtd/keypulse/ci.yaml?branch=main&label=tests)](https://github.com/jonathan-gtd/keypulse/actions/workflows/ci.yaml) [![License](https://img.shields.io/github/license/jonathan-gtd/keypulse?color=lightgrey)](LICENSE)

[![lang en](https://img.shields.io/badge/lang-en-lightgrey)](README.md) [![lang fr](https://img.shields.io/badge/lang-fr-41BDF5)](README.fr.md)

</div>

---

[WhatPulse](https://whatpulse.org) et [MouseTracks](https://github.com/huntfx/MouseTracks) montrent comment vous utilisez clavier et souris, mais gardent les données dans leur application ou leur cloud. keypulse est un petit agent Windows qui fait le comptage et confie les chiffres à ce que vous faites déjà tourner — Telegraf, InfluxDB, TimescaleDB, Grafana.

- ⌨️ **Touches, clics, molette, distance de la souris et temps actif**, chaque minute.
- 🪟 **Par application** — combien vous tapez dans votre éditeur, cliquez dans votre jeu.
- 🔥 **Heatmaps** — où passe le curseur, où vous cliquez et (en option) quelles touches vous utilisez.
- 🎮 **Conçu pour ne pas coûter une image** — aucun hook, priorité basse, cœurs efficaces. [Détails](#performances)
- 🔒 **Des comptes, pas un journal** — jamais ce que vous tapez. [Détails](#vie-privée)

---

## Installation

**1. Téléchargez** `keypulse-<version>-setup.exe` depuis la [dernière version](https://github.com/jonathan-gtd/keypulse/releases/latest) et lancez-le. Il s'installe pour votre utilisateur seulement (sans droits administrateur) et démarre keypulse à l'ouverture de session.

> L'installateur n'est pas encore signé : Windows SmartScreen peut demander confirmation (*Informations complémentaires → Exécuter quand même*). Les sommes de contrôle sont publiées avec chaque version.

**2. Donnez-lui une destination.** Par défaut, un `socket_listener` Telegraf sur le même ordinateur :

```toml
[[inputs.socket_listener]]
  service_address = "udp://127.0.0.1:8094"
  data_format = "influx"
```

Ensuite, n'importe quelle sortie Telegraf convient : InfluxDB, PostgreSQL/TimescaleDB, MQTT, Prometheus… Exemple TimescaleDB dans [docs/telegraf.md](docs/telegraf.md). keypulse peut aussi écrire directement dans InfluxDB 2 (`output.url` dans la configuration).

---

## Configuration

`%APPDATA%\keypulse\config.yaml`, écrit par l'installateur. Tous les réglages sont facultatifs — voir [config.example.yaml](config.example.yaml).

| Réglage | Défaut | |
|---|---|---|
| `interval` | `60s` | Fréquence d'envoi, alignée sur l'horloge |
| `per_app` | `true` | Répartition par application au premier plan |
| `per_key` | `false` | Compte de chaque touche |
| `heatmap.enabled` | `true` | Positions du curseur et des clics |
| `heatmap.cell_size` | `40` | Grille de la heatmap, en pixels |
| `mouse_dpi` | `0` | DPI de votre souris, pour la distance en mètres |
| `output.url` | `udp://127.0.0.1:8094` | `udp://` (Telegraf), `http(s)://` (listener Telegraf, InfluxDB 2) |
| `output.token` | | Jeton InfluxDB |
| `host` | nom de l'ordinateur | Valeur du tag `host` |

Relancez keypulse après une modification.

---

## Données

Format *line protocol*, un lot par intervalle, rien n'est envoyé s'il ne s'est rien passé. Le détail des mesures, tags et champs est dans le [README anglais](README.md#data).

---

## Performances

- **Raw Input, pas un hook.** Un hook clavier ou souris s'insère entre vos périphériques et toutes les applications : s'il est lent, l'entrée est retardée. keypulse s'abonne à [Raw Input](https://learn.microsoft.com/windows/win32/inputdev/raw-input) : Windows lui remet une copie de chaque événement, le jeu ne l'attend jamais.
- **Priorité basse et [EcoQoS](https://devblogs.microsoft.com/performance-diagnostics/introducing-ecoqos/)** — sur les processeurs hybrides, Windows le garde sur les cœurs efficaces.
- **Aucune interrogation en boucle.** L'application au premier plan vient d'un événement Windows ; la position du curseur est lue au plus 10 fois par seconde, et seulement si la souris a bougé.
- **Aucune allocation par événement**, un petit paquet réseau par intervalle.

Mesuré sur un i9-14900KF pendant 30 secondes d’usage ordinaire de la souris, démarrage compris : 0,6 % d’un cœur (0,02 % du processeur entier) et 48 Mo de mémoire.

---

## Vie privée

keypulse compte, il n'enregistre pas. Les totaux ne disent pas quelles touches ont été pressées. `per_key` enregistre combien de fois chaque touche a été pressée par intervalle — jamais leur ordre — mais sur des intervalles courts cela peut laisser deviner ce qui a été tapé : c'est pour cela qu'il est désactivé par défaut. Tout part uniquement vers l'adresse que vous configurez.

## Licence

[MIT](LICENSE)
