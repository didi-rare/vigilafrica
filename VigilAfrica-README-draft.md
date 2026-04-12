# VigilAfrica

**VigilAfrica** is an open-source effort to make natural event data more understandable and locally relevant across Africa.

The project aims to translate raw geospatial event signals, such as floods, wildfires, storms, and drought-related activity, into administrative areas people actually recognize, including countries, states, provinces, and local government areas.

Instead of asking users to interpret coordinates and satellite metadata, VigilAfrica is being built to answer a simpler question:

**What is happening near me?**

---

## Why this project exists

Across Africa, environmental and natural event data is often available globally, but not always presented in ways that are easy for local communities, responders, researchers, journalists, or civic organizations to act on.

VigilAfrica exists to bridge that gap by combining:

- global event feeds such as NASA EONET
- African administrative boundary context
- simple, location-aware user experiences

The long-term goal is to help people discover meaningful event activity around them without needing geospatial expertise.

---

## Current project status

VigilAfrica is currently in the **early prototype stage**.

This repository contains the foundation for a monorepo-based implementation with:

- a **Go backend** for ingestion and API services
- a **React + Vite + TypeScript frontend**
- **OpenSpec-based project governance**
- CI/CD workflow scaffolding for development, staging, and production branches

At this stage, the project is focused on defining the product, validating architecture, and building the first end-to-end prototype.

---

## Initial scope

The first milestone is to prove a simple but useful workflow:

1. ingest natural event data
2. normalize it into a usable internal format
3. enrich it with local administrative context
4. expose it through an API
5. display it in a simple web experience

The early prototype will prioritize a smaller, more credible scope before expanding across more countries and event types.

---

## What VigilAfrica is intended to support

Over time, VigilAfrica is intended to support use cases such as:

- local environmental awareness
- community preparedness
- NGO and humanitarian situational awareness
- logistics and field operations planning
- public-interest journalism and research

---

## Architecture direction

The platform is being designed around a few core responsibilities:

- **Ingestion**  
  Pull event data from trusted upstream sources such as NASA EONET

- **Normalization**  
  Convert source-specific event payloads into a common internal structure

- **Geospatial enrichment**  
  Map event coordinates or geometries to African administrative boundaries

- **API delivery**  
  Serve filtered, location-aware event data to clients

- **User interface**  
  Present event information through a simple African-focused dashboard and map experience

---

## Technology direction

- **Backend:** Go
- **Frontend:** React + Vite + TypeScript
- **Spatial data:** PostgreSQL + PostGIS
- **Location detection:** MaxMind GeoLite2
- **Governance/specs:** OpenSpec

---

## Repository structure

```text
/api          Backend services and ingestion logic
/web          Frontend application
/.github      Workflows and repository automation
openspec.yaml OpenSpec project configuration
package.json  Root scripts and shared tooling
```

---

## Branch strategy

The repository currently uses the following branch intent:

- `development` → active development
- `main` → staging
- `releases` → production

---

## Near-term goals

The immediate goal is to deliver a working prototype that can:

- ingest a small set of event data
- enrich those events with local geography
- expose a basic API
- render a simple dashboard experience

---

## Contributing

VigilAfrica is intended to be a community-driven project over time.

As the prototype matures, contribution guides, issue templates, and implementation documentation will be expanded to make onboarding easier.

For now, contributions, feedback, and collaboration ideas are welcome through GitHub Issues.

---

## License

VigilAfrica is licensed under the **Apache License 2.0**.

This supports open collaboration while remaining friendly to public-interest, research, and commercial reuse.

---

## Contact

Maintained by **[@didi-rare](https://github.com/didi-rare)**.

For collaboration or project discussions, use GitHub Issues or reach out directly via email.
