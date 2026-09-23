# Participant Step-by-Step User Journey

This comprehensive user guide details every stage of the participant experience on IvyTicketing, from event discovery to crossing the finish line and claiming your finisher certificate.

---

## 1. Account Management and Profile Verification

1. **Sign In**: Access your athlete dashboard at `/auth/login`.
2. **Profile Accuracy**: Ensure your medical details, emergency contact, date of birth, and jersey size are up to date. Sizing cannot be modified after registration closes.
3. **Session Management**: IvyTicketing maintains secure sessions using encrypted JWT access tokens with automatic silent refresh.

---

## 2. Event Discovery and Registration Selection

1. **Browse Events**: Visit the events catalog to view upcoming races, discipline categories, venue maps, and entry requirements.
2. **Registration Modes**:
   - **Direct Mode**: Click **Register Now** to proceed straight to category selection.
   - **War Queue Mode**: For high-traffic flash sales, clicking register enters you into the virtual waiting room.
   - **Ballot Mode**: Click **Enter Ballot** to submit an application. No upfront payment is charged until you are selected as a winner.

---

## 3. The Waiting Room Experience (War Queue)

When registering for high-demand events:

```
[####################>              ] 62%
Your position in line: 420
Estimated wait time: ~1 minute 15 seconds
```

- **Keep Your Tab Open**: The waiting room maintains an active connection and polls status every 2 seconds.
- **Do Not Refresh**: Refreshing does not lose your queue position, but keeping the tab open guarantees immediate redirection when you are released.
- **Admission Window**: Once released, your status changes to `ALLOWED`. You have exactly **5 minutes** to submit your checkout order before your admission token expires.

---

## 4. Ballot (Lottery) Journey

1. **Submit Entry**: During the ballot window, choose your category and submit your athlete details. Your entry is recorded with status `APPLIED`.
2. **Withdrawing**: You can withdraw your application at any time while the ballot window remains open.
3. **Draw Results**: When the draw takes place, you will receive an email and SMS notification:
   - **Selected (Winner)**: You are granted an exclusive `access_grant` with a strict payment deadline (e.g. 48 hours). Log in and click **Complete Registration** to pay.
   - **Waitlisted**: If winners fail to pay within the deadline, waitlisted applicants are automatically promoted in rank order. Keep an eye on your email for promotion notices.

---

## 5. Checkout and Payment Execution

1. **Registration Form**: Fill in required event fields (e.g. custom BIB name, running club, medical conditions).
2. **Coupon Codes**: Enter any promotional discount code and click **Apply**.
3. **Inventory Hold (15 Minutes)**: Submitting the order creates a `PENDING_PAYMENT` order and locks your ticket slot in the database for 15 minutes.
4. **Payment Options**:
   - **QRIS**: Scan the dynamic QR code with any e-wallet or mobile banking app.
   - **Virtual Account**: Transfer to the provided unique account number via ATM or mobile banking.
   - **Credit / Debit Card**: Submit card details verified via 3DS OTP.
5. **Instant Confirmation**: As soon as your payment clears, your order transitions to `PAID` and digital tickets are generated.

---

## 6. Managing Your Digital Tickets

1. Navigate to **My Tickets** (`/tickets/my-tickets`).
2. Each ticket displays:
   - Event and category name
   - Participant name and assigned BIB number (once generated)
   - Cryptographic HMAC QR code
3. You can download an offline PDF copy of your registration confirmation ticket.

---

## 7. Racepack Collection (Expo)

### Booking a Pickup Slot
To avoid queues at the race expo:
1. Open your ticket in the dashboard.
2. Under **Racepack Pickup**, select an available date and time slot (e.g., "Friday, 14:00 - 16:00").
3. Your slot confirmation is linked to your ticket.

### Authorizing a Proxy Collection
If you cannot attend the expo in person:
1. Open your ticket and click **Authorize Proxy**.
2. Enter the proxy person's full legal name and National Identity / Passport Number.
3. Upload a signed power of attorney letter.
4. Your proxy must present their physical photo ID and your digital ticket QR code at the expo counter.

---

## 8. Post-Race Results and Finisher Certificates

1. Following race completion, official times are published under **Results** (`/results`).
2. Enter your BIB number or athlete name to view:
   - Net Chip Time and Gross Gun Time
   - Overall, Gender, and Category Ranking
   - Intermediate Split Times (e.g. 5K, 10K, 21K, 30K)
3. Click **Download Finisher Certificate** to generate a high-resolution, personalized certificate with verified race splits.
