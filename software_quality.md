# Software Quality

**Created:** 21. sep. 2026

**Last revised:** 24. sep. 2026

After you have setup (a) code quality tool(s) and gone through the issues, your group should create a brief document that answers the following questions:

We have created the setup, but have not discussed or acted on the results yet

**Do you agree with the findings?**

24th september: SonarQube found 21 issues. 14/21 is html, 5/21 is legacy code based and the last two is security in our deploy.sh file

Findings we agreed on: "Missing" id on input fields for username and password. We agreed that this could be done for individual styling, but since we want the same styling for the two inputs we ignored it.

Findings we **did not** agree on: The legacy code failed a security cehck in regards to password hashing. We didn't fix it since it was in the legacy code and not our go code.

**Which ones did you fix?**

Double square brackets in our deploy.sh, 

**Which ones did you ignore?**

Legacy code: hashing, network binding, disclosing flask keys, CSRF

Go code: id and labels for input fields

**Why?**

We currently (24th september) decided not to change the id nor make labels in our html since it's working and we didnt have a reason as for now. However we are aware of it moving forward.