# User guide

Todo is a small todo list for the browser. This guide shows how to use it: items, due dates,
reminders and labels. Open the app at https://toodoo.fly.dev.

## Get started

### Create an account

1. Open https://toodoo.fly.dev and click **Sign up**.
2. Type a username: 3 to 32 letters, digits, `-` or `_`.
3. Type a password. The page shows the shortest length that it accepts.
4. Click **Create account**. Your list opens.

There is no email address. If you forget your password, ask the admin of the app.

### Log in and log out

1. Type your username and password.
2. If you use your own computer, select **Keep me logged in**. Then you stay logged in for 30
   days. You can be logged in on at most 20 devices or browsers at the same time. A new login
   ends the oldest one.
3. Click **Log in**.

![The login page](guide-images/login.png "the login page with the fields Username and Password, the box “Keep me logged in”, the link to Sign up, and the links to the user guide and the privacy policy.")

To log out, click **Log out** at the top right of your list.

### Your account

Click **Account** at the top right of your list. On the Account page you can:

- Change your password. You must type your current password first. After the change, the app
  logs you out on your other devices. After 5 wrong current passwords, wait 15 minutes before
  you try again.
- Delete your account. Type your username to confirm. This deletes your username, your
  password, all your items and labels, and all your logins at once. You cannot undo it. The
  daily backups of the server keep this data for up to 5 more days. After that, it is gone.

If you use only the Android app, delete your toodoo account on the web: log in at
https://toodoo.fly.dev, click **Account**, and type your username to confirm. The Android app
has no delete button.

The privacy policy at https://toodoo.fly.dev/privacy tells you which data the app stores, why,
and for how long. The login, sign-up and Account pages link to it.

## Items

![The list with items, due dates and labels](guide-images/list.png "the list with five items. The first item is overdue and has the postpone buttons. The last item is done.")

### Add an item

1. Type the task in the field "What needs to be done?".
2. Press Enter or click **Add**. The item goes to the end of the list.

An item text has at most 500 characters. You can have at most 5,000 items, done or not.

If nothing on the page has the focus, you can start to type at once. The text goes into the
add field.

### Edit an item

1. Click the text of the item. The text becomes a field.
2. Change the text.
3. Press Enter to save, or Esc to cancel. If you click somewhere else, the app saves the text.

![An item in edit mode](guide-images/edit.png "the item “Water the plants” in edit mode. Under the field is the hint “Enter to save · Esc to cancel”.")

### Check off and delete

- Click the box on the left of an item to mark it done. Click it again to mark it not done.
- Click the × on the right of an item to delete it.

### Done items

- **Hide done** hides the done items. **Show done** shows them again. The app remembers your
  choice.
- **Clear done** deletes all done items. The app asks first, because you cannot undo it. If the
  number of done items changes before you click **Delete** (for example in another tab), the
  app deletes nothing and asks again with the new number.

![The question of Clear done](guide-images/clear-done.png "the question after a click on Clear done. “Delete 1 item” deletes the done items. Cancel keeps them.")

## Order of the items

- Drag an item by its grip (⋮⋮) on the left to a new place.
- With the keyboard: press Tab until the grip has the focus, then press the up arrow or the down
  arrow.

Overdue items always stay at the top. You cannot move them, and you cannot move other items
above them.

## Due dates

Each item has a small calendar icon under its text.

1. Click the calendar icon. A pop-up opens.
2. Pick a day.
3. If you want a time, type it in the second field. Type only the digits: `1430` becomes 14:30.
   The app uses the 24-hour clock.
4. Click **Save**.

![The due date pop-up](guide-images/due-popup.png "the due date pop-up of the item “Send the invoice”, with the day, the time 10:00, Save and Remove date.")

- If you leave the time empty, the item is due all day.
- To change a date, click the calendar icon or the due date text.
- To remove a date, open the pop-up and click **Remove date**.
- Esc closes the pop-up and does not save.

The app shows the times in the time zone of your browser.

### Overdue items

When the due time passes, the date text becomes red ("Overdue since …"), and the item moves to
the top of the list.

## Reminders

The browser can show a notification when an item is due.

1. When you have an item with a due date, the list shows **Turn on notifications**. Click it.
2. The browser asks for permission. Click **Allow**.

The app shows each notification only one time, also if you have more than one tab open. If the
list says "Notifications are blocked in your browser settings", allow notifications for this
site in your browser settings.

### Postpone

An overdue item shows the buttons **+5 min**, **+10 min**, **+15 min** and **+30 min**. Click one
to move the due time to that many minutes from now.

![An overdue item with the postpone buttons](guide-images/overdue.png "an overdue item. The red text shows since when it is overdue. Under it are the four postpone buttons.")

## Labels

Labels such as "work" or "home" put your items into groups. Each label has a color.

### Add and remove labels

1. Click the small **+** under the text of an item. The label list opens.
2. Click a label name to add it to the item. A check shows on its line.
3. Click a name with a check to remove that label from the item.
4. The list stays open, so you can click more names. Press Esc or click outside the list to close
   it.

![The label list](guide-images/label-list.png "the label list of the item “Send the invoice”. The item has the label “work”, so “work” has a check.")

To find a label, type a part of its name in the search field at the top of the list.

You can also remove a label from an item without the list: point at the label on the item and
click its ×. On a phone the × always shows.

### Create a label

1. In the label list, type the name of the new label.
2. The last line shows **Create “name”**. Its color circle shows the color that the label will
   get. To pick another color, click the circle first (see below).
3. Click **Create “name”**. The app makes the label and adds it to the item.

### Change the color of a label

1. In the label list, click the color circle at the start of the label's line. A color wheel
   shows in the pop-up.
2. Click or drag on the wheel. The angle sets the color, and the distance from the middle sets
   how strong it is. You can also use the arrow keys.
3. Click **Done** to save the color, or **Cancel** to keep the old color.

![The color wheel](guide-images/color-wheel.png "the color wheel of the label “work”. The small circle is the current color. Preview shows the label with it.")

### Delete a label

1. In the label list, point at the line of the label and click the **×** at the right end. On a
   phone the × always shows.
2. The line asks "Delete “name”?" and shows how many items use the label. Click **Delete**.

The app removes the label from all items. The items stay.

![The question before a label is deleted](guide-images/label-delete.png "the question before the label “urgent” is deleted. One item has this label. The item stays.")

### Show only the items of one label

Click a label on an item. The list shows only the items with that label, and the top of the list
shows "Label: name". A new item that you add here gets this label.

To see all items again, click the × next to "Label: name".

![The list with only the items of one label](guide-images/filter.png "only the items with the label “work”. “Label: work” has an × that shows all items again.")

### Limits

- A label name has at most 30 characters and no comma.
- An item has at most 5 labels.
- You can have at most 50 labels.

## On a phone

The app works in the browser of a phone. On a touch screen:

- The calendar icon, the + and the × on a label always show, and they are larger.
- The × to delete a label in the label list always shows.

![The list on a phone](guide-images/phone.png "the list in the browser of a phone, 375 pixels wide. The × on each label always shows.")

## Android app

There is also an app for Android phones (Android 8.0 and newer). It shows the same list as the
web, so a change on the phone shows in the browser and the other way round.

- **Install:** the app is not in Google Play. Get the APK file (`toodoo.apk`) and open it on the
  phone; Android asks once to allow the install. How to build it: `docs/android.md` in the repo.
- **Log in** with your username and password. You stay logged in while you open the app at
  least every two weeks. Sign-up, the password change and deleting the account are on the web;
  the menu's "Account (web)" opens that page.
- **Privacy policy:** "Privacy policy" on the login screen and in the menu (⋮) opens
  https://toodoo.fly.dev/privacy in the browser.
- **The list:** swipe a row to the right to mark it done (on a done row, the same swipe marks it
  not done again). Swipe a row to the left to delete it. Both show Undo for a few seconds, and
  the change is saved when Undo goes away. Hold a row still for half a second until it lifts,
  then drag it to change the order (overdue items stay at the top). Pull down to load the list
  again. The first time, a blue hint above the list explains this; "Got it" closes it.
- **Without swiping:** the item sheet has a "Mark as done" button. With TalkBack, each row has
  the actions Mark as done, Move up, Move down and Delete.
- **An item:** tap it. The sheet has the text, the due date, Postpone, the labels, "Mark as
  done", and Delete. Tap "Date" to pick the day. Then tap the time button to pick the time on a 24-hour
  clock. The keyboard icon in the time picker lets you type the time instead. "All day" removes
  the time. Save sends the text and the due date.
- **Labels for a new item:** when you tap the add field, a row of your labels shows between the
  field and the keyboard. The labels with the most items come first; swipe the row to see more.
  Tap a label to select it, and tap it again to clear it. "Add" gives the new item the selected
  labels, then the row clears. With the label filter on, its label is already selected. To
  create a new label, use "All labels" in the item sheet or the label list from the top bar.
- **Labels of an item:** the sheet shows a row of all your labels. The labels of the item come
  first and are selected; the others follow, most used first. Tap a label to put it on the item
  or take it off, at once (no Save). The order stays while the sheet is open.
- **Labels:** "All labels" in the sheet and the tag button at the top open the same list,
  "Labels". Type in the field "Search or create…" to find a label. If no label has that name,
  "Create" makes it. From the top bar you can create a label before any item has it. Tap a color
  dot for the color wheel. The × deletes a label.
- **Show only the items of one label:** open "Labels" with the tag button at the top. The text
  under the field says "Tap a label to show only its items." Tap a label to do that. From "All
  labels" in the sheet, a tap on a label adds it to the item or removes it.
- **Reminders:** the phone reminds you when an item is due, also when the app is closed. The
  reminder has the buttons Done and Postpone 10 min. The app asks for permission to notify when
  you set your first due date. Without a connection the app shows the last list it loaded; changes
  need the connection.

![The list in the Android app](guide-images/android-list.png "the list in the Android app, with five open items and one done item. The tag button at the top opens the label list.")

![The item sheet](guide-images/android-itemSheet.png "the item sheet of “Send invoice to Kowalski”. The labels “money” and “work” are selected, and the others follow.")

![The label list from the item sheet](guide-images/android-labelPicker.png "the label list from “All labels” in the item sheet. The search “he” finds two labels, and “Create” can make “he”.")

![The label list from the top bar](guide-images/android-labelPickerFilter.png "the label list from the tag button at the top. A tap on a label shows only its items.")

![Adding an item with labels](guide-images/android-addLabelsKeyboard.png "the Android app while you add an item. The keyboard is open, and the row of labels is between the add field and the keyboard. “home” and “health” are selected.")

## Keys

| Where | Key | What it does |
| --- | --- | --- |
| Anywhere in the list | A letter or a digit | Starts a new item in the add field |
| Add field | Enter | Adds the item |
| Item in edit mode | Enter | Saves the text |
| Item in edit mode | Esc | Cancels the change |
| Grip (⋮⋮) | Up arrow, down arrow | Moves the item |
| Due date pop-up | Enter | Saves the date |
| Due date pop-up | Esc | Closes the pop-up |
| Label list, search field | Down arrow | Goes to the first label name |
| Label list | Up arrow, down arrow | Moves between the label names |
| Label list, search field | Enter | Adds or removes the first label, or creates the typed label |
| Label list | Esc | Closes the list |
| Color wheel | Arrow keys | Changes the color |
| Color wheel | Enter | Done |
| Color wheel | Esc | Cancel |
