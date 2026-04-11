# VigilAfrica 🌍 (African Natural Event Tracker)

VigilAfrica is an open-source, community-driven platform designed to provide intuitive, hyper-localized insights into natural events across the African continent. By leveraging NASA's EONET (Earth Observatory Natural Event Tracker) API and local administrative boundaries, VigilAfrica empowers farmers, logistics providers, NGOs, and public safety officials to track and respond to environmental changes in near real-time.

---

## 🚀 Vision
Our mission is to democratize access to geospatial event data for every African nation. VigilAfrica translates satellite metadata into local context—shifting the focus from global coordinates to specific LGAs, states, and provinces "Happening Near You."

---

## ✨ Key Features
- **Intuitive Localization Dashboard**: Automatically detects your general location (Country/State) via IP to show relevant local events.
- **Interactive Map**: A tailored African-centric map with filters for Wildfires, Floods, Storms, and Drought.
- **Geospatial Enrichment**: Maps NASA coordinates to local administrative names using PostGIS and high-quality boundary data.
- **OpenSpec Governance**: Managed via [OpenSpec](https://openspec.ai) to ensure documentation and implementation never drift.

---

## 🛠 Tech Stack
- **Backend**: Go (API Server & Spatial Ingestor)
- **Frontend**: React + Vite + TypeScript (Interactive Dashboard)
- **Database**: PostgreSQL with PostGIS extension
- **Geocoding**: Local MaxMind GeoLite2 for low-latency IP lookups
- **Hosting**: Designed for Vercel (Frontend/Functions) and managed PostgreSQL providers.

---

## 📂 Repository Structure (Monorepo)
```text
/api            # Go Backend (API, Ingestor, Enrichment)
/web            # React Frontend (Dashboard, Interactive Map)
/openspec       # Project Specifications & OpenSpec Governance
/infra          # Deployment configurations (Vercel, Docker)
```

---

## 🤝 Community & Contributing
VigilAfrica is built for and by the African community. We welcome contributions for all 54 nations!

- See [CONTRIBUTING.md](CONTRIBUTING.md) (coming soon) for guidelines.
- Participate in discussions via GitHub Issues.

---

## 📜 License: Apache 2.0
VigilAfrica is released under the **Apache License 2.0**. 

### Why Apache 2.0?
- **Commercial Friendly**: You are free to use VigilAfrica for commercial applications.
- **Patent Protection**: The license includes an explicit grant of patent rights from contributors.
- **Transparency**: Ensures the platform remains an open public good for the continent.

### Expectations
We expect contributors and users to respect the attribution requirements of the Apache 2.0 license. If you redistribute or build upon this work, you must include a copy of the license and clear attribution to the VigilAfrica project.

---

## 💰 Support the Mission
VigilAfrica is a volunteer-led project. To support our hosting costs and data enrichment efforts, consider contributing to our maintenance fund:

[**Support us on GoFundMe**](https://gofundme.com/vigilafrica-placeholder)

---

## 📞 Contact
Maintained by **DidiPepple**. Reach out via GitHub Issues for collaborations or inquiries.
