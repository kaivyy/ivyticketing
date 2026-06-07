PRD — Race Registration & Event Ticketing Platform
Version: 1.0
Author: Varin
Status: Draft
Target Market: Running Events, Marathon, Trail Run, Cycling, Triathlon, Fun Run, Expo, Seminar, Concert

1. Vision
Membangun platform pendaftaran event generasi berikutnya yang:
Anti-chaos saat war tiket
Transparan dan adil
Mendukung ballot dan waiting list
Multi organizer (SaaS)
White-label
Custom form tanpa coding
Multi payment gateway
Enterprise-ready
Mampu menangani 100.000+ concurrent users
Positioning:
"Race registration platform built for high-demand events."

2. Product Goals
Primary Goals
Registrasi event yang cepat
Queue yang transparan
Tidak ada overselling
Tidak ada antrian reset
Tidak ada double payment
Tidak ada double racepack pickup
Secondary Goals
White-label organizer
Revenue platform
Multi gateway payment
Multi tenant
Self-service organizer

3. User Roles
Platform Owner
Role:
Super Admin
Akses:
Semua organizer
Semua transaksi
Revenue platform
System configuration

Organizer
Role:
Owner
Manager
Finance
Customer Service
Race Director
Akses:
Event milik sendiri

Staff
Role:
Racepack Team
Volunteer
Check-In Staff
Akses terbatas.

Participant
Peserta event.

4. System Architecture
Frontend:
Astro
Backend:
Go
Database:
PostgreSQL
Cache:
Redis / DragonflyDB
Storage:
Cloudflare R2
CDN:
Cloudflare
Queue:
Cloudflare Waiting Room
Cloudflare Durable Objects
Monitoring:
Prometheus
Grafana
Loki
Sentry
Deployment:
Docker
Kubernetes (optional)

5. Multi Tenant Structure
Platform
└── Organizer
└── Event
└── Category
└── Participant
Every major table contains:
organization_id
Examples:
organizations
events
participants
orders
payments
forms

6. Event Management
Organizer can create:
Running event
Marathon
Trail Run
Cycling
Triathlon
Expo
Seminar
Concert
Fields:
Event name
Description
Banner
Logo
Schedule
Venue
Maps
FAQ
Terms
Waiver

7. Event Categories
Examples:
42K
21K
10K
5K
Kids Dash
Per category:
Price
Capacity
Registration open
Registration close
Waiting list
Ballot enabled
Queue enabled
BIB prefix

8. Registration Modes
Normal Sale
First come first served.

Queue Sale
Waiting room enabled.
Features:
Persistent queue
Reconnect support
Queue history
Queue status

Ballot
Registration period.
Then:
Random selection
Winner announcement
Payment window

Invitation Only
Access code required.

9. Queue System
Core Requirements:
Queue never resets
Refresh safe
Mobile sleep safe
Network reconnect safe
Display:
Position
Estimated time
Status
Progress
Queue token stored:
Cookie
Database
Durable Objects
Admin Controls:
Pause queue
Resume queue
Throttle release

10. Anti-Bot System
Cloudflare Turnstile
Rate limiting
Device fingerprint
IP reputation
Account limits
Email verification
Phone verification
Optional KYC

11. Registration Form Builder
Organizer can create custom fields.
Field Types:
Text
Email
Phone
Number
Date
Dropdown
Radio
Checkbox
File Upload
Textarea
Conditional Logic Supported.
Examples:
Blood Type
Emergency Contact
Emergency Phone
Medical Conditions
Community
Strava Link
Passport Number
Required / Optional

12. Participant Dashboard
Features:
My Events
My Orders
My Tickets
My Certificates
My Results
My Profile

13. Orders
States:
Draft
Pending
Paid
Expired
Cancelled
Refunded
Features:
Invoice
History
Status Timeline

14. Payment System
Supported:
Duitku
Xendit
Midtrans
Future:
Stripe
PayPal
Organizer chooses:
One gateway
Multiple gateways
Features:
QRIS
VA
Credit Card
E-Wallet

15. Payment Reliability
Requirements:
Idempotency
Webhook retries
Reconciliation
Payment logs
Fallback gateway
No double orders
No double charges

16. Atomic Inventory System
Never:
Check stock -> create order
Always:
Atomic decrement
Guarantees:
No overselling
No race condition
No duplicate slots

17. Coupon System
Features:
Fixed discount
Percentage
Quota
Date limits
Category restrictions
Community codes
Invitation codes

18. Merchandise Module
Additional products:
Jersey
Jacket
Cap
Bag
Inventory tracking
Size tracking
Bundle support

19. BIB Management
Modes:
Sequential
Randomized
Uploaded
Examples:
42K = A0001
21K = B0001
10K = C0001
Assignment Trigger:
PAID only
Not Pending.

20. QR Ticket System
Every participant receives:
E-ticket
QR Code
Signed Token
QR Contains:
ticket_id
event_id
signature
No sensitive data.

21. Racepack Module
Status:
Ready
Picked Up
Proxy Pickup
Cancelled
Features:
Pickup scheduling
QR verification
Counter assignment
Pickup history

22. Pickup Slot System
Participant chooses:
Friday
10-11
Friday
11-12
Saturday
10-11
Each slot has quota.
Goal:
Avoid crowding.

23. Racepack Scanner
Platform:
Web
PWA
Functions:
Scan QR
Display participant
Confirm pickup
Duplicate detection
Offline mode
Sync later

24. Proxy Pickup
Features:
Authorization
Proxy QR
Audit trail
History

25. Check-In System
Race day check-in.
Scan BIB.
Status:
Checked In
Started
Finished
DNF

26. Results Integration
Future module.
Import:
CSV
API
Chip timing vendors
Display:
Ranking
Gender rank
Age group rank
Certificates

27. Communication System
Email
WhatsApp
Broadcast
Transactional messages
Templates customizable.

28. Notifications
Registration
Payment success
Payment failed
Queue access
Ballot result
Racepack reminder
Race reminder
Results available

29. Organizer Dashboard
Dashboard
Events
Categories
Participants
Orders
Payments
Forms
Coupons
Merchandise
Queue
Ballot
Racepack
Reports
Broadcast
Settings

30. Staff Roles
Owner
Manager
Finance
Customer Service
Volunteer
Racepack Team
Check-In Team
Permissions configurable.

31. Super Admin Dashboard
Manage:
Organizers
Subscriptions
Revenue
Transactions
Support
Logs
System Health
Gateways
Pricing

32. White Label
Organizer can use:
Custom logo
Custom colors
Custom domain
Custom email sender
Custom templates
Example:
register.jakartamarathon.id

33. Reporting
Participants
Revenue
Sales by category
Conversion rate
Payment success rate
Queue analytics
Racepack analytics
Coupon analytics
Exports:
CSV
Excel

34. Public Status Page
Displays:
System status
Payment status
Queue status
Incident reports
Last update time
Purpose:
Reduce panic during war.

35. Reliability Requirements
Target:
99.95% uptime
No queue reset
No stock oversell
No duplicate payment
No duplicate racepack pickup

36. Performance Targets
Concurrent users:
100,000+
Peak traffic:
500,000+
Queue latency:
<100ms
API p95:
<300ms
Page load:
<2 seconds

37. Security
WAF
DDoS Protection
Encryption
Signed QR Tokens
Audit Logs
Role-Based Access
Webhook Validation

38. Monetization
SaaS Subscription
Starter
Professional
Enterprise

Platform Fee
Per ticket

Hybrid
Subscription
+
Platform Fee

Enterprise Services
White Label
Dedicated Infrastructure
Dedicated Queue
Dedicated Support
API Access
Custom Integrations

39. Future Roadmap
V2
Mobile App
Native Scanner App
Timing Integration
Certificate Generator
Sponsor Marketplace
Community Management
Affiliate Program
Referral System
Insurance Upsell
Travel Package Integration
Hotel Integration
Race Ranking System
National Event Network

40. Success Metrics
Payment Success Rate > 99%
Queue Complaint Rate < 1%
Racepack Processing < 10 seconds
System Availability > 99.95%
Organizer Retention > 90%
Participant Satisfaction > 4.8/5
NPS > 60
