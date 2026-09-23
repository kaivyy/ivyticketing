# Organizer Operations and Event Management Guide

This comprehensive manual covers end-to-end event operations for race organizers, from initial category and form design to war-day queue management, expo racepack logistics, and official timing results publication.

---

## 1. Event Setup and Inventory Configuration

### Creating Ticket Categories
1. In your organization workspace, navigate to **Events** -> **[Event Name]** -> **Categories**.
2. Define quotas and pricing for each distance or division:
   - **Total Quota**: Authoritative hard limit.
   - **Price and Currency**: Base ticket price in local currency (e.g. IDR).
   - **Order Limits**: Set maximum tickets allowed per checkout (typically 1 for competitive races).
   - **Age Rules**: Minimum and maximum age on race day.

### Designing Custom Registration Forms
1. Open the **Registration Forms** tab.
2. Add custom questions required for race operations:
   - Text inputs: Custom BIB Name, Running Club / Team Name.
   - Dropdown selections: Jersey / T-shirt Size, Blood Type.
   - Emergency contacts: Contact Name and Phone Number.
   - Legal waivers: Mandatory liability and medical waiver checkboxes.

---

## 2. Managing High-Traffic War Sales

During high-demand ticket releases, operational control is maintained through the **Queue Operations Dashboard**:

1. **Pre-Sale Configuration**:
   - Set Registration Mode to `WAR_QUEUE` or `RANDOMIZED_QUEUE`.
   - Configure **Release Rate**: Default recommendation is 100 participants per 10 seconds.
   - Set **Checkout Window**: 5 minutes (`300s`).
2. **War-Room Monitoring**:
   - Track live queue depth, admitted user flow, and conversion rates.
3. **Dynamic Adjustments**:
   - **Slow Down**: If payment gateways experience increased latency, decrease the release rate to 50 users per 10s.
   - **Emergency Pause**: If a critical configuration error or gateway outage occurs, click **Pause Queue**. Currently admitted users can finish checkout, but no new users enter.

---

## 3. Ballot (Lottery) Management

1. **Configuring Draw Windows**:
   - Navigate to **Ballots** -> **Create Ballot Draw**.
   - Specify application start and end dates.
   - Set total winning capacity (e.g. 5,000 slots).
   - Configure payment deadline duration (e.g. 48 hours from announcement).
2. **Executing the Draw**:
   - Once the application window closes, click **Execute Random Draw**.
   - The system randomizes applicant entries, selects winners, marks remaining applicants as waitlisted, and issues single-use access grants.
   - Review draw logs before clicking **Publish Results**.
3. **Monitoring Conversions**:
   - Track winner payment completion percentage.
   - The automated expirer worker will lapse unpaid winners and promote waitlisted applicants automatically every 60 seconds.

---

## 4. Access Pools and Priority Quotas

For corporate sponsors, running clubs, and VIP athletes:
1. Navigate to **Access & Quotas** -> **Access Pools**.
2. Create a dedicated pool (e.g. "Title Sponsor - 500 Slots").
3. Generate single-use or multi-use access codes.
4. Participants enter these codes to unlock private registration windows and bypass general queues.

---

## 5. BIB Number Allocation

1. Navigate to **Tickets & BIBs** -> **BIB Management**.
2. **Automatic Sequential Allocation**:
   - Click **Batch Assign BIBs**.
   - Configure category prefixes and ranges:
     - Full Marathon: `1001` to `2500`
     - Half Marathon: `3001` to `6000`
   - Click **Run Auto-Assignment**. The system locks the sequence and numbers all paid tickets.
3. **Manual Overrides**:
   - Search for a specific athlete by name or email.
   - Click **Edit BIB** to assign custom numbers for elite athletes or pacers.

---

## 6. Racepack Expo Logistics

1. **Slot Configuration**:
   - Under **Racepack Setup**, define expo dates, counter locations, and hourly slot capacities.
2. **Operating Expo Counters**:
   - Staff open the scanner interface at `/scan` or use the PWA scanner.
   - Scan the athlete's digital ticket QR code.
   - The screen displays category, t-shirt size, and assigned BIB number.
   - Hand over the race kit and click **Confirm Hand-off** (`status = PICKED_UP`).
3. **Proxy Verification**:
   - If a representative collects, the scanner displays the uploaded authorization letter and proxy ID for on-site inspection.
4. **Handling Disputes at Problem Desk**:
   - For size swaps or damaged bibs, transfer the participant to the Problem Desk counter.
   - Staff log a case in `racepack_problem_cases`, make necessary system updates, and record the resolution.

---

## 7. Results and Finisher Certificates

1. **Importing Timing Data**:
   - In **Results Management**, click **Import Results**.
   - Upload the official timing CSV file containing columns: `bib`, `gun_time_ms`, `chip_time_ms`, `splits`.
2. **Reviewing Rankings**:
   - System automatically calculates category, gender, and overall rankings.
   - Review any flagged anomalies (`DNF`, `DSQ`, `OTL`).
3. **Finisher Certificates**:
   - Upload a high-resolution certificate template background image.
   - Position dynamic text coordinates (Athlete Name, BIB, Category, Net Time, Overall Rank).
   - Publish results to make certificates instantly downloadable by participants.
