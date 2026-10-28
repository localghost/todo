# Privacy policy

This policy is effective from 2026-10-28.

This policy tells you what data toodoo stores about you, why, and for how long. It also tells you
what your rights are.

## Who is responsible

Zbigniew Kostrzewa, Poland, is responsible for your data (the "controller" in the GDPR). In this
policy, "we" means Zbigniew Kostrzewa.

Contact: zkostrzewa.dev@gmail.com

This policy covers:

- the web app at https://toodoo.fly.dev
- the toodoo Android app, when it connects to https://toodoo.fly.dev

The Android app can also connect to a server that another person runs. In that case, the person who
runs that server is responsible for the data on it, and this policy does not cover it.

## Summary

- We store only what the app needs to work: your account, your items and your labels.
- We do not ask for your email address or your real name.
- We do not show ads, and we do not track you.
- We do not use analytics or crash reporting.
- We do not sell your data, and we do not send you emails.
- You can delete your account and all your data at any time.

## What data we store, and why

| Data | Why we store it | Legal basis (GDPR) | How long |
| --- | --- | --- | --- |
| Your username | To identify your account and to log you in | Art. 6(1)(b): we need it to give you the service | Until you delete your account |
| A hash of your password (made with Argon2id) | To check your password when you log in. We never store the password itself. | Art. 6(1)(b) | Until you delete your account |
| The time your account was made | To manage accounts | Art. 6(1)(b) | Until you delete your account |
| Your items: the text, done or not, the order, the due date, when you made and changed them, when a reminder was sent | To show and keep your list | Art. 6(1)(b) | Until you delete the item or your account |
| Your labels: the name and the color | To show and keep your labels | Art. 6(1)(b) | Until you delete the label or your account |
| Sessions: a hash of your log-in token, and when it starts and ends | To keep you logged in | Art. 6(1)(b) | 12 hours. With "Keep me logged in", or in the Android app: 30 days, renewed when you use the app. We delete ended sessions every hour. |
| Your IP address, in the memory of the server | To limit the number of log-in and sign-up attempts, which protects accounts from attacks | Art. 6(1)(f): our interest in a safe service | 15 minutes to 1 hour. It is lost when the server restarts. |
| Your IP address in the server log, only when the server blocks a log-in or a sign-up | To find and stop attacks | Art. 6(1)(f) | As long as Fly.io keeps its logs (see "Who else gets your data") |

You must give a username and a password to use toodoo. Without them, we cannot make an account for you.

We do not use your data for profiling. We do not make automated decisions about you.

## Cookies

The web app uses only two cookies. Both are needed for the app to work, so we do not ask for your
consent:

- The session cookie keeps you logged in. It ends when your session ends.
- The `todo_tz` cookie holds the time zone of your browser, for example `Europe/Warsaw`. The server
  uses it to show due dates in your time. It stays for 1 year.

We do not use cookies for ads, analytics or tracking.

## Data on your phone (Android app)

The Android app stores these on your phone, in storage that only the app can read:

- the server address, your username and your log-in token
- a copy of your last list, so that you can see it without a network connection

When you log out, the app deletes the token and the copy of your list. When you uninstall the app,
Android deletes all of this data.

The app turns off Android backups, so this data does not go to a cloud backup.

The app asks for these permissions:

- internet, to connect to the server
- notifications, alarms and "run at start-up", to show reminders for your due dates

The phone makes the reminders itself. No push service gets your data.

If you install the app from Google Play, Google gets data about the installation. The Google Play
privacy policy covers this data, not this policy.

## Who else gets your data

- Fly.io, Inc. (USA) runs the servers. The database is in Frankfurt, Germany. Fly.io handles every
  request to toodoo, with your IP address, and stores the server log. Fly.io makes a copy of the
  database each day and keeps each copy for 5 days. Fly.io works for us and only on our
  instructions (a "processor" in the GDPR).
- Google (Google Fonts). The log-in, sign-up and account pages and the user guide load fonts from
  fonts.googleapis.com. So your browser sends your IP address and browser details to Google. The
  legal basis is Art. 6(1)(f): our interest in a page that is easy to read. You can find more at
  https://policies.google.com/privacy

Fly.io and Google are companies in the USA. So your data can go to the USA. Google takes part in the
EU-U.S. Data Privacy Framework. The data processing terms of Fly.io use the Standard Contractual
Clauses of the European Commission.

We do not give your data to anyone else. The only exception is when a law or a court order tells us
to.

## How long we keep your data

We keep your account, items and labels until you delete them. The table above shows the time for
each type of data.

When you delete your account, we delete your username, password hash, items, labels and sessions at
once. The daily copies of the database still hold the data for up to 5 more days. After that, the
data is gone.

## Your rights

Under the GDPR, you have these rights:

- to get a copy of your data (access)
- to correct your data
- to delete your data
- to limit how we use your data (restriction)
- to get your data in a common format that a computer can read (portability)
- to object when we use your data because of our interest (Art. 6(1)(f))

You can change your items, labels and password yourself in the app.

You can delete your account yourself. Log in at https://toodoo.fly.dev, open "Account", and select
"Delete my account and all items". In the Android app, there is no delete button yet. You can use
the web app, or send us an email.

For all other rights, send an email to zkostrzewa.dev@gmail.com and give your username. We answer
within 1 month.

You can also complain to the Polish data protection authority: Prezes Urzędu Ochrony Danych
Osobowych (UODO), ul. Stawki 2, 00-193 Warszawa, https://uodo.gov.pl

## Age

You must be 16 or older to use toodoo.

We do not knowingly store data about children under 16. If you think that a child under 16 has an
account, send us an email, and we will delete the account.

## Security

- We store only a hash of your password (Argon2id), never the password itself.
- We store only a hash of your log-in token, never the token itself.
- The app works only over HTTPS, so your data is encrypted on the network.
- The session cookie is not visible to scripts on the page, and it goes only over HTTPS.

If a data breach puts your rights at risk, we will tell UODO, as the GDPR requires. We do not have
your email address. So if the breach puts your rights at high risk, we will tell you with a notice
on https://toodoo.fly.dev

## Changes to this policy

If we change this policy, we will put the new text here with a new date at the top.
