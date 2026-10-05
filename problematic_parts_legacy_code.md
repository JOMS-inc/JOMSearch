# Problems with the legacy code

**SQL Injection**  
User input is concatenated into sql queries. People with ill intentions, could steal values from the db or attack the database in a similar way.

**No salts**
When it comes to password hashing, so identical passwords gives identical hashes and can be cracked with a rainbow table

**Python 2.7 (outdated version of python)**  
The python version is not supported officially. Future exploits will not be patched. It's harder to maintain

**Old dependencies**  
Old dependencies can make it harder to implement new features, since they aren't supported anymore. It can also lead to security risks, like with the old python version

**No test**  
If we were to make changes to the codebase, we might only spot it if the app crashes. Besides that bugs could also be introduced.

**Missing comments**  
For other developers the codebase can be hard to navigate around, especially being so outdated. Adding no comments on top of that, makes it even harder to navigate.

**Mismatch with readme and reality**  
The readme file says: "Start a development server on port `8080`:"
But the server is actually running on 8081

